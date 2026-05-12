package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

type nodeStorage interface {
	GetNode(ctx context.Context, id int64) (pg.NodeRow, error)
	GetNodeInfo(ctx context.Context, nodeID int64) (pg.NodeInfoRow, bool, error)
}

type nodeResponse struct {
	ID              int64             `json:"id"`
	LogID           string            `json:"log_id"`
	GUID            string            `json:"guid"`
	Type            string            `json:"type"`
	Desc            string            `json:"desc,omitempty"`
	SystemImageGUID string            `json:"system_image_guid,omitempty"`
	PortGUID        string            `json:"port_guid,omitempty"`
	SwitchInfo      map[string]string `json:"switch_info,omitempty"`
	SystemInfo      map[string]string `json:"system_info,omitempty"`
	SharpInfo       map[string]string `json:"sharp_info,omitempty"`
}

func nodeHandler(storage nodeStorage, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("node_id"), 10, 64)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid node_id")

			return
		}

		node, err := storage.GetNode(r.Context(), id)
		if err != nil {
			if errors.Is(err, pg.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "node not found")

				return
			}

			log.Error("get node", "err", err, "node_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		info, hasInfo, err := storage.GetNodeInfo(r.Context(), id)
		if err != nil {
			log.Error("get node info", "err", err, "node_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		resp := nodeResponse{
			ID:              node.ID,
			LogID:           node.LogID.String(),
			GUID:            node.GUID,
			Type:            node.Type,
			Desc:            node.Desc,
			SystemImageGUID: node.SystemImageGUID,
			PortGUID:        node.PortGUID,
		}

		if hasInfo {
			resp.SwitchInfo = info.SwitchInfo
			resp.SystemInfo = info.SystemInfo
			resp.SharpInfo = info.SharpInfo
		}

		writeJSON(w, log, http.StatusOK, resp)
	}
}
