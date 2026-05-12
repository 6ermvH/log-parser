//go:build !integration

package http

import (
	"errors"
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

func TestNodeHandler_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocknodeQuery(ctrl)

	logID := uuid.New()
	q.EXPECT().GetNodeDetails(gomock.Any(), int64(42)).Return(service.NodeDetails{
		ID:         42,
		LogID:      logID,
		GUID:       "0xsw",
		Type:       domain.NodeTypeSwitch,
		Desc:       "SW1",
		SwitchInfo: map[string]string{"k": "v"},
	}, nil)

	h := nodeHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/42", nil)
	req.SetPathValue("node_id", "42")

	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp nodeResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, int64(42), resp.ID)
	assert.Equal(t, "0xsw", resp.GUID)
	assert.Equal(t, domain.NodeTypeSwitch, resp.Type)
	assert.Equal(t, "v", resp.SwitchInfo["k"])
}

func TestNodeHandler_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocknodeQuery(ctrl)

	q.EXPECT().GetNodeDetails(gomock.Any(), int64(999)).Return(service.NodeDetails{}, service.ErrNotFound)

	h := nodeHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/999", nil)
	req.SetPathValue("node_id", "999")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNodeHandler_BadID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocknodeQuery(ctrl)

	h := nodeHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/abc", nil)
	req.SetPathValue("node_id", "abc")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNodeHandler_InternalError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocknodeQuery(ctrl)

	q.EXPECT().GetNodeDetails(gomock.Any(), int64(1)).Return(service.NodeDetails{}, errors.New("db down"))

	h := nodeHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/node/1", nil)
	req.SetPathValue("node_id", "1")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
