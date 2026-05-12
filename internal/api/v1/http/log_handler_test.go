//go:build !integration

package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/api/v1/http/mocks"
	"github.com/6ermvH/log-parser/internal/service"
)

func TestLogHandler_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocklogMetaQuery(ctrl)

	id := uuid.New()
	uploaded := time.Now()
	q.EXPECT().GetLogMeta(gomock.Any(), id).Return(service.LogMeta{
		ID:         id,
		Status:     "ok",
		UploadedAt: uploaded,
		NodesCount: 4,
		PortsCount: 100,
	}, nil)

	h := logMetaHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/"+id.String(), nil)
	req.SetPathValue("log_id", id.String())

	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp logResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, id.String(), resp.ID)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, 4, resp.NodesCount)
	assert.Equal(t, 100, resp.PortsCount)
}

func TestLogHandler_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocklogMetaQuery(ctrl)

	id := uuid.New()
	q.EXPECT().GetLogMeta(gomock.Any(), id).Return(service.LogMeta{}, service.ErrNotFound)

	h := logMetaHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/"+id.String(), nil)
	req.SetPathValue("log_id", id.String())

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLogHandler_BadUUID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocklogMetaQuery(ctrl)

	h := logMetaHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/not-a-uuid", nil)
	req.SetPathValue("log_id", "not-a-uuid")

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogHandler_InternalError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	q := mocks.NewMocklogMetaQuery(ctrl)

	id := uuid.New()
	q.EXPECT().GetLogMeta(gomock.Any(), id).Return(service.LogMeta{}, errors.New("db down"))

	h := logMetaHandler(q, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/log/"+id.String(), nil)
	req.SetPathValue("log_id", id.String())

	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
