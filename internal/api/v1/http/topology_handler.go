package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/service"
)

type topologyQuery interface {
	GetTopology(ctx context.Context, logID uuid.UUID) (service.Topology, error)
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

// topologyHandler returns the full topology of a log.
//
//	@Summary		Get topology
//	@Description	Returns nodes, ports and connections (edges) for the given log.
//	@Tags			topology
//	@Produce		json
//	@Param			log_id	path		string	true	"Log UUID"
//	@Success		200		{object}	topologyResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/api/v1/topology/{log_id} [get]
func topologyHandler(q topologyQuery, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID, err := uuid.Parse(r.PathValue("log_id"))
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid log_id")

			return
		}

		topo, err := q.GetTopology(r.Context(), logID)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "log not found")

				return
			}

			log.Error("get topology", "err", err, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusOK, buildTopologyResponse(topo))
	}
}

func buildTopologyResponse(t service.Topology) topologyResponse {
	respNodes := make([]topologyNode, 0, len(t.Nodes))
	for _, n := range t.Nodes {
		respNodes = append(respNodes, topologyNode{
			ID:    n.ID,
			LogID: n.LogID.String(),
			GUID:  n.GUID,
			Type:  n.Type,
			Desc:  n.Desc,
		})
	}

	respPorts := make([]portResponse, 0, len(t.Ports))
	for _, p := range t.Ports {
		respPorts = append(respPorts, toPortResponse(p))
	}

	respEdges := make([]topologyEdge, 0, len(t.Edges))
	for _, e := range t.Edges {
		respEdges = append(respEdges, topologyEdge{PortAID: e.PortAID, PortBID: e.PortBID})
	}

	return topologyResponse{Nodes: respNodes, Ports: respPorts, Edges: respEdges}
}
