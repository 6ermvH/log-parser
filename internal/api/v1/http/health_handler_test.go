//go:build !integration

package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/api/v1/http/mocks"
)

func TestHealthHandler_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	checker := mocks.NewMockhealthChecker(ctrl)

	checker.EXPECT().Ping(gomock.Any()).Return(nil)

	h := healthHandler(checker, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok"`)
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	checker := mocks.NewMockhealthChecker(ctrl)

	checker.EXPECT().Ping(gomock.Any()).Return(errors.New("connection refused"))

	h := healthHandler(checker, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"unhealthy"`)
}
