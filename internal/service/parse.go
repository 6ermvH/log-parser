package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/domain"
)

type parseRepo interface {
	InsertProcessingLog(ctx context.Context, id uuid.UUID) error
	SaveDomainLog(ctx context.Context, id uuid.UUID, dlog domain.Log) error
	MarkLogFailed(ctx context.Context, id uuid.UUID, message string) error
}

type logParser interface {
	Parse(path string) (domain.Log, error)
}

type ParseService struct {
	parser logParser
	repo   parseRepo
	log    *slog.Logger
}

func NewParseService(p logParser, r parseRepo, log *slog.Logger) *ParseService {
	return &ParseService{parser: p, repo: r, log: log}
}

type ParseResult struct {
	LogID    uuid.UUID
	ParseErr error
}

func (s *ParseService) Run(ctx context.Context, path string) (ParseResult, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return ParseResult{}, fmt.Errorf("generate uuid: %w", err)
	}

	if iErr := s.repo.InsertProcessingLog(ctx, id); iErr != nil {
		s.log.Error("insert processing log failed", "log_id", id, "err", iErr)

		return ParseResult{}, fmt.Errorf("insert processing: %w", iErr)
	}

	parseStart := time.Now()
	dlog, parseErr := s.parser.Parse(path)
	parseDuration := time.Since(parseStart)

	if parseErr != nil {
		s.log.Warn("parse failed",
			"log_id", id,
			"path", path,
			"parse_duration_ms", parseDuration.Milliseconds(),
			"err", parseErr,
		)

		if mErr := s.repo.MarkLogFailed(ctx, id, parseErr.Error()); mErr != nil {
			s.log.Error("mark log failed", "log_id", id, "err", mErr)
		}

		return ParseResult{LogID: id, ParseErr: parseErr}, nil
	}

	saveStart := time.Now()
	sErr := s.repo.SaveDomainLog(ctx, id, dlog)
	saveDuration := time.Since(saveStart)

	if sErr != nil {
		s.log.Error("save log failed",
			"log_id", id,
			"save_duration_ms", saveDuration.Milliseconds(),
			"err", sErr,
		)

		if mErr := s.repo.MarkLogFailed(ctx, id, "save failed: "+sErr.Error()); mErr != nil {
			s.log.Error("mark log failed after save error", "log_id", id, "err", mErr)
		}

		return ParseResult{}, fmt.Errorf("save: %w", sErr)
	}

	s.log.Info("log parsed",
		"log_id", id,
		"path", path,
		"parse_duration_ms", parseDuration.Milliseconds(),
		"save_duration_ms", saveDuration.Milliseconds(),
		"nodes_count", len(dlog.Nodes),
		"ports_count", countPorts(dlog.Nodes),
		"connections_count", len(dlog.Connections),
	)

	return ParseResult{LogID: id}, nil
}

func countPorts(nodes []domain.Node) int {
	total := 0
	for _, n := range nodes {
		total += len(n.Ports)
	}

	return total
}
