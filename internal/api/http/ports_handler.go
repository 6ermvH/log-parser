package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

type portsStorage interface {
	NodeExists(ctx context.Context, id int64) (bool, error)
	ListPortsByNode(ctx context.Context, nodeID int64) ([]pg.PortRow, error)
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

func portsHandler(storage portsStorage, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodeID, err := strconv.ParseInt(r.PathValue("node_id"), 10, 64)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid node_id")

			return
		}

		exists, err := storage.NodeExists(r.Context(), nodeID)
		if err != nil {
			log.Error("node exists", "err", err, "node_id", nodeID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		if !exists {
			writeError(w, log, http.StatusNotFound, "node not found")

			return
		}

		rows, err := storage.ListPortsByNode(r.Context(), nodeID)
		if err != nil {
			log.Error("list ports", "err", err, "node_id", nodeID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		ports := make([]portResponse, 0, len(rows))
		for _, p := range rows {
			ports = append(ports, toPortResponse(p))
		}

		writeJSON(w, log, http.StatusOK, portsResponse{Ports: ports})
	}
}

func toPortResponse(p pg.PortRow) portResponse {
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
