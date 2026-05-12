package service

import (
	"context"
	"fmt"
	"log/slog"

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
		return ParseResult{}, fmt.Errorf("insert processing: %w", iErr)
	}

	dlog, parseErr := s.parser.Parse(path)
	if parseErr != nil {
		if mErr := s.repo.MarkLogFailed(ctx, id, parseErr.Error()); mErr != nil {
			s.log.Error("mark log failed", "err", mErr, "log_id", id)
		}

		return ParseResult{LogID: id, ParseErr: parseErr}, nil
	}

	if sErr := s.repo.SaveDomainLog(ctx, id, dlog); sErr != nil {
		if mErr := s.repo.MarkLogFailed(ctx, id, "save failed: "+sErr.Error()); mErr != nil {
			s.log.Error("mark log failed after save error", "err", mErr, "log_id", id)
		}

		return ParseResult{}, fmt.Errorf("save: %w", sErr)
	}

	return ParseResult{LogID: id}, nil
}
