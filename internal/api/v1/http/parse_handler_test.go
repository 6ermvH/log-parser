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

func runParseHandler(t *testing.T, body string, setup func(*mocks.MockparseSubmitter)) *httptest.ResponseRecorder {
	t.Helper()

	ctrl := gomock.NewController(t)
	svc := mocks.NewMockparseSubmitter(ctrl)

	if setup != nil {
		setup(svc)
	}

	h := parseHandler(svc, discardLogger(), t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/parse", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h(w, req)

	return w
}

func TestParseHandler_Accepted(t *testing.T) {
	t.Parallel()

	logID := uuid.New()
	w := runParseHandler(t, `{"path":"log.zip"}`, func(s *mocks.MockparseSubmitter) {
		s.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(logID, nil)
	})

	require.Equal(t, http.StatusAccepted, w.Code)

	var resp parseResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, logID.String(), resp.LogID)
}

func TestParseHandler_SubmitError(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":"log.zip"}`, func(s *mocks.MockparseSubmitter) {
		s.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, errors.New("db down"))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestParseHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseHandler_EmptyPath(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":""}`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseHandler_InputNotFound(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":"missing.zip"}`, func(s *mocks.MockparseSubmitter) {
		s.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, parser.ErrInputNotFound)
	})

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, "input file not found", resp.Error)
}

func TestParseHandler_InputNotZip(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":"not_a_zip.txt"}`, func(s *mocks.MockparseSubmitter) {
		s.EXPECT().Submit(gomock.Any(), gomock.Any()).Return(uuid.Nil, parser.ErrInputNotZip)
	})

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp errorResponse

	decodeJSON(t, w.Body, &resp)
	assert.Equal(t, "input is not a valid zip archive", resp.Error)
}

func TestParseHandler_PathOutsideDataDir(t *testing.T) {
	t.Parallel()

	w := runParseHandler(t, `{"path":"../etc/passwd"}`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
