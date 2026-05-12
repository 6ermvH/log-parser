//go:build !integration

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/api/http/mocks"
	"github.com/6ermvH/log-parser/internal/service"
)

func TestPortsHandler_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMockportsQuery(ctrl)

	q.EXPECT().ListPortsForNode(gomock.Any(), int64(1)).Return([]service.Port{
		{ID: 10, NodeID: 1, Num: 1, State: 4},
		{ID: 11, NodeID: 1, Num: 2, State: 1},
	}, nil)

	h := portsHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/port/1", nil)
	req.SetPathValue("node_id", "1")

	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp portsResponse

	decodeJSON(t, w.Body, &resp)
	require.Len(t, resp.Ports, 2)
	assert.Equal(t, 4, resp.Ports[0].State)
}

func TestPortsHandler_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMockportsQuery(ctrl)

	q.EXPECT().ListPortsForNode(gomock.Any(), int64(999)).Return(nil, service.ErrNotFound)

	h := portsHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/port/999", nil)
	req.SetPathValue("node_id", "999")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPortsHandler_BadID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMockportsQuery(ctrl)

	h := portsHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/port/foo", nil)
	req.SetPathValue("node_id", "foo")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
