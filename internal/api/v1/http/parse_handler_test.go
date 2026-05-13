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
	"github.com/6ermvH/log-parser/internal/parser"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func decodeJSON(t *testing.T, body io.Reader, dst any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(body).Decode(dst))
}

func TestParseHandler_Accepted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	logID := uuid.New()
	svc.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(logID, nil)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"log.zip"}`))
	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var resp parseResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, logID.String(), resp.LogID)
}

func TestParseHandler_SubmitError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	svc.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, errors.New("db down"))

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
	svc := mocks.NewMockparseSubmitter(ctrl)

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
	svc := mocks.NewMockparseSubmitter(ctrl)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":""}`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseHandler_InputNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	svc.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, parser.ErrInputNotFound)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"missing.zip"}`))
	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, "input file not found", resp.Error)
}

func TestParseHandler_InputNotZip(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	svc.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, parser.ErrInputNotZip)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"not_a_zip.txt"}`))
	w := httptest.NewRecorder()
	h(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, "input is not a valid zip archive", resp.Error)
}

func TestParseHandler_PathOutsideDataDir(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse",
		bytes.NewBufferString(`{"path":"../etc/passwd"}`))
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
