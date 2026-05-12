package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

type queryRepo interface {
	GetLog(ctx context.Context, id uuid.UUID) (pg.LogMeta, error)
	CountByLog(ctx context.Context, logID uuid.UUID) (pg.Counts, error)
	ListNodes(ctx context.Context, logID uuid.UUID) ([]pg.NodeRow, error)
	ListPortsByLog(ctx context.Context, logID uuid.UUID) ([]pg.PortRow, error)
	ListPortsByNode(ctx context.Context, nodeID int64) ([]pg.PortRow, error)
	ListConnections(ctx context.Context, logID uuid.UUID) ([]pg.ConnectionRow, error)
	GetNode(ctx context.Context, id int64) (pg.NodeRow, error)
	GetNodeInfo(ctx context.Context, nodeID int64) (pg.NodeInfoRow, bool, error)
	NodeExists(ctx context.Context, id int64) (bool, error)
}

type QueryService struct {
	repo queryRepo
}

func NewQueryService(repo queryRepo) *QueryService {
	return &QueryService{repo: repo}
}

func (s *QueryService) GetLogMeta(ctx context.Context, id uuid.UUID) (LogMeta, error) {
	meta, err := s.repo.GetLog(ctx, id)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			return LogMeta{}, ErrNotFound
		}

		return LogMeta{}, fmt.Errorf("get log: %w", err)
	}

	counts, err := s.repo.CountByLog(ctx, id)
	if err != nil {
		return LogMeta{}, fmt.Errorf("count: %w", err)
	}

	return LogMeta{
		ID:           meta.ID,
		Status:       meta.Status,
		UploadedAt:   meta.UploadedAt,
		NodesCount:   counts.Nodes,
		PortsCount:   counts.Ports,
		ErrorMessage: meta.ErrorMessage,
	}, nil
}

func (s *QueryService) GetTopology(ctx context.Context, logID uuid.UUID) (Topology, error) {
	if _, err := s.repo.GetLog(ctx, logID); err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			return Topology{}, ErrNotFound
		}

		return Topology{}, fmt.Errorf("get log: %w", err)
	}

	nodes, err := s.repo.ListNodes(ctx, logID)
	if err != nil {
		return Topology{}, fmt.Errorf("list nodes: %w", err)
	}

	ports, err := s.repo.ListPortsByLog(ctx, logID)
	if err != nil {
		return Topology{}, fmt.Errorf("list ports: %w", err)
	}

	edges, err := s.repo.ListConnections(ctx, logID)
	if err != nil {
		return Topology{}, fmt.Errorf("list connections: %w", err)
	}

	return buildTopology(nodes, ports, edges), nil
}

func (s *QueryService) GetNodeDetails(ctx context.Context, id int64) (NodeDetails, error) {
	n, err := s.repo.GetNode(ctx, id)
	if err != nil {
		if errors.Is(err, pg.ErrNotFound) {
			return NodeDetails{}, ErrNotFound
		}

		return NodeDetails{}, fmt.Errorf("get node: %w", err)
	}

	info, hasInfo, err := s.repo.GetNodeInfo(ctx, id)
	if err != nil {
		return NodeDetails{}, fmt.Errorf("get node info: %w", err)
	}

	out := NodeDetails{
		ID:              n.ID,
		LogID:           n.LogID,
		GUID:            n.GUID,
		Type:            n.Type,
		Desc:            n.Desc,
		SystemImageGUID: n.SystemImageGUID,
		PortGUID:        n.PortGUID,
	}

	if hasInfo {
		out.SwitchInfo = info.SwitchInfo
		out.SystemInfo = info.SystemInfo
		out.SharpInfo = info.SharpInfo
	}

	return out, nil
}

func (s *QueryService) ListPortsForNode(ctx context.Context, nodeID int64) ([]Port, error) {
	exists, err := s.repo.NodeExists(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("node exists: %w", err)
	}

	if !exists {
		return nil, ErrNotFound
	}

	rows, err := s.repo.ListPortsByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("list ports: %w", err)
	}

	out := make([]Port, 0, len(rows))
	for _, p := range rows {
		out = append(out, toServicePort(p))
	}

	return out, nil
}

func buildTopology(nodes []pg.NodeRow, ports []pg.PortRow, edges []pg.ConnectionRow) Topology {
	sNodes := make([]TopologyNode, 0, len(nodes))
	for _, n := range nodes {
		sNodes = append(sNodes, TopologyNode{
			ID:    n.ID,
			LogID: n.LogID,
			GUID:  n.GUID,
			Type:  n.Type,
			Desc:  n.Desc,
		})
	}

	sPorts := make([]Port, 0, len(ports))
	for _, p := range ports {
		sPorts = append(sPorts, toServicePort(p))
	}

	sEdges := make([]Edge, 0, len(edges))
	for _, e := range edges {
		sEdges = append(sEdges, Edge{PortAID: e.PortAID, PortBID: e.PortBID})
	}

	return Topology{Nodes: sNodes, Ports: sPorts, Edges: sEdges}
}

func toServicePort(p pg.PortRow) Port {
	return Port{
		ID:            p.ID,
		NodeID:        p.NodeID,
		Num:           p.Num,
		GUID:          p.GUID,
		State:         p.State,
		PhyState:      p.PhyState,
		LinkSpeedActv: p.LinkSpeedActv,
		LinkWidthActv: p.LinkWidthActv,
		LID:           p.LID,
		Raw:           p.Raw,
	}
}
