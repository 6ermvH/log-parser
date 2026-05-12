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
