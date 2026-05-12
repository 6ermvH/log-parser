package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

type topologyStorage interface {
	GetLog(ctx context.Context, id uuid.UUID) (pg.LogMeta, error)
	ListNodes(ctx context.Context, logID uuid.UUID) ([]pg.NodeRow, error)
	ListPortsByLog(ctx context.Context, logID uuid.UUID) ([]pg.PortRow, error)
	ListConnections(ctx context.Context, logID uuid.UUID) ([]pg.ConnectionRow, error)
}

type topologyNode struct {
	ID    int64  `json:"id"`
	LogID string `json:"log_id"`
	GUID  string `json:"guid"`
	Type  string `json:"type"`
	Desc  string `json:"desc,omitempty"`
}

type topologyEdge struct {
	PortAID int64 `json:"port_a_id"`
	PortBID int64 `json:"port_b_id"`
}

type topologyResponse struct {
	Nodes []topologyNode `json:"nodes"`
	Ports []portResponse `json:"ports"`
	Edges []topologyEdge `json:"edges"`
}

func topologyHandler(storage topologyStorage, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID, err := uuid.Parse(r.PathValue("log_id"))
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid log_id")

			return
		}

		if _, gErr := storage.GetLog(r.Context(), logID); gErr != nil {
			if errors.Is(gErr, pg.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "log not found")

				return
			}

			log.Error("get log", "err", gErr, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		nodes, err := storage.ListNodes(r.Context(), logID)
		if err != nil {
			log.Error("list nodes", "err", err, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		ports, err := storage.ListPortsByLog(r.Context(), logID)
		if err != nil {
			log.Error("list ports", "err", err, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		edges, err := storage.ListConnections(r.Context(), logID)
		if err != nil {
			log.Error("list connections", "err", err, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusOK, buildTopologyResponse(nodes, ports, edges))
	}
}

func buildTopologyResponse(nodes []pg.NodeRow, ports []pg.PortRow, edges []pg.ConnectionRow) topologyResponse {
	respNodes := make([]topologyNode, 0, len(nodes))
	for _, n := range nodes {
		respNodes = append(respNodes, topologyNode{
			ID:    n.ID,
			LogID: n.LogID.String(),
			GUID:  n.GUID,
			Type:  n.Type,
			Desc:  n.Desc,
		})
	}

	respPorts := make([]portResponse, 0, len(ports))
	for _, p := range ports {
		respPorts = append(respPorts, toPortResponse(p))
	}

	respEdges := make([]topologyEdge, 0, len(edges))
	for _, e := range edges {
		respEdges = append(respEdges, topologyEdge{PortAID: e.PortAID, PortBID: e.PortBID})
	}

	return topologyResponse{Nodes: respNodes, Ports: respPorts, Edges: respEdges}
}
