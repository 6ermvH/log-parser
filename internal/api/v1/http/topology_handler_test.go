//go:build !integration

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/api/v1/http/mocks"
	"github.com/6ermvH/log-parser/internal/domain"
	"github.com/6ermvH/log-parser/internal/service"
)

func TestTopologyHandler_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocktopologyQuery(ctrl)

	logID := uuid.New()
	q.EXPECT().GetTopology(gomock.Any(), logID).Return(service.Topology{
		Nodes: []service.TopologyNode{
			{ID: 1, LogID: logID, GUID: "0xa", Type: domain.NodeTypeHost},
		},
		Ports: []service.Port{
			{ID: 10, NodeID: 1, Num: 1, State: 4},
		},
	}, nil)

	h := topologyHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/"+logID.String(), nil)
	req.SetPathValue("log_id", logID.String())

	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp topologyResponse

	decodeJSON(t, w.Body, &resp)
	require.Len(t, resp.Nodes, 1)
	require.Len(t, resp.Ports, 1)
	assert.Equal(t, "0xa", resp.Nodes[0].GUID)
}

func TestTopologyHandler_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocktopologyQuery(ctrl)

	logID := uuid.New()
	q.EXPECT().GetTopology(gomock.Any(), logID).Return(service.Topology{}, service.ErrNotFound)

	h := topologyHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/"+logID.String(), nil)
	req.SetPathValue("log_id", logID.String())

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTopologyHandler_BadUUID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocktopologyQuery(ctrl)

	h := topologyHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/topology/bad", nil)
	req.SetPathValue("log_id", "bad")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
