//go:build !integration

package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/api/v1/http/mocks"
	"github.com/6ermvH/log-parser/internal/service"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func decodeJSON(t *testing.T, body io.Reader, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(body).Decode(dst))
}

func TestParseHandler_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	logID := uuid.New()
	svc.EXPECT().Run(gomock.Any(), gomock.Any()).Return(service.ParseResult{LogID: logID}, nil)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"log.zip"}`))
	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp parseResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, logID.String(), resp.LogID)
	assert.Empty(t, resp.Error)
}

func TestParseHandler_ParseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	logID := uuid.New()
	parseErr := errors.New("broken zip")
	svc.EXPECT().Run(gomock.Any(), gomock.Any()).Return(service.ParseResult{LogID: logID, ParseErr: parseErr}, nil)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"log.zip"}`))
	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp parseResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, logID.String(), resp.LogID)
	assert.Equal(t, "broken zip", resp.Error)
}

func TestParseHandler_ServiceSystemError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	svc.EXPECT().Run(gomock.Any(), gomock.Any()).Return(service.ParseResult{}, errors.New("db down"))

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"log.zip"}`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestParseHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseHandler_EmptyPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":""}`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseHandler_PathOutsideDataDir(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseRunner(ctrl)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"../etc/passwd"}`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
