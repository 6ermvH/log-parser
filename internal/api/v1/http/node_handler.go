package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/6ermvH/log-parser/internal/service"
)

type nodeQuery interface {
	GetNodeDetails(ctx context.Context, id int64) (service.NodeDetails, error)
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

// nodeHandler returns full details for one node.
//
//	@Summary		Get node details
//	@Description	Returns node attributes and optional info blocks (switch_info / system_info / sharp_info).
//	@Tags			node
//	@Produce		json
//	@Param			node_id	path		integer	true	"Node id"
//	@Success		200		{object}	nodeResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/api/v1/node/{node_id} [get]
func nodeHandler(q nodeQuery, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("node_id"), 10, 64)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid node_id")

			return
		}

		details, err := q.GetNodeDetails(r.Context(), id)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "node not found")

				return
			}

			log.Error("get node details", "err", err, "node_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusOK, nodeResponse{
			ID:              details.ID,
			LogID:           details.LogID.String(),
			GUID:            details.GUID,
			Type:            details.Type,
			Desc:            details.Desc,
			SystemImageGUID: details.SystemImageGUID,
			PortGUID:        details.PortGUID,
			SwitchInfo:      details.SwitchInfo,
			SystemInfo:      details.SystemInfo,
			SharpInfo:       details.SharpInfo,
		})
	}
}
