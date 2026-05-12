package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/6ermvH/log-parser/internal/service"
)

type portsQuery interface {
	ListPortsForNode(ctx context.Context, nodeID int64) ([]service.Port, error)
}

type portResponse struct {
	ID            int64             `json:"id"`
	NodeID        int64             `json:"node_id"`
	Num           int               `json:"port_num"`
	GUID          string            `json:"guid,omitempty"`
	State         int               `json:"state"`
	PhyState      int               `json:"phy_state"`
	LinkSpeedActv int               `json:"link_speed_actv"`
	LinkWidthActv int               `json:"link_width_actv"`
	LID           int               `json:"lid"`
	Raw           map[string]string `json:"raw,omitempty"`
}

type portsResponse struct {
	Ports []portResponse `json:"ports"`
}

// portsHandler returns all ports of a node.
//
//	@Summary		List node ports
//	@Description	Returns the list of ports for the given node, ordered by port number.
//	@Tags			port
//	@Produce		json
//	@Param			node_id	path		integer	true	"Node id"
//	@Success		200		{object}	portsResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/api/v1/port/{node_id} [get]
func portsHandler(q portsQuery, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID, err := strconv.ParseInt(r.PathValue("node_id"), 10, 64)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid node_id")

			return
		}

		ports, err := q.ListPortsForNode(r.Context(), nodeID)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "node not found")

				return
			}

			log.Error("list ports", "err", err, "node_id", nodeID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		resp := portsResponse{Ports: make([]portResponse, 0, len(ports))}
		for _, p := range ports {
			resp.Ports = append(resp.Ports, toPortResponse(p))
		}

		writeJSON(w, log, http.StatusOK, resp)
	}
}

func toPortResponse(p service.Port) portResponse {
	return portResponse{
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
