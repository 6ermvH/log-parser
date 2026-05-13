//go:build !integration

package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/domain"
	"github.com/6ermvH/log-parser/internal/parser"
	"github.com/6ermvH/log-parser/internal/service"
	"github.com/6ermvH/log-parser/internal/service/mocks"
)

const shutdownTimeout = 2 * time.Second

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitShutdown(t *testing.T, svc *service.ParseService) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	require.NoError(t, svc.Shutdown(ctx))
}

func newTestParseSvc(t *testing.T) (*service.ParseService, *mocks.MockparseRepo, *mocks.MocklogParser) {
	t.Helper()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parserMock := mocks.NewMocklogParser(ctrl)
	svc := service.NewParseService(parserMock, repo, discardLogger())

	return svc, repo, parserMock
}

func TestParseService_Submit_Success(t *testing.T) {
	t.Parallel()

	svc, repo, parserMock := newTestParseSvc(t)

	dlog := domain.Log{Nodes: []domain.Node{{GUID: "0xa", Type: domain.NodeTypeHost}}}

	parserMock.EXPECT().Preflight("/data/log.zip").Return(nil)
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parserMock.EXPECT().Parse("/data/log.zip").Return(dlog, nil)
	repo.EXPECT().SaveDomainLog(gomock.Any(), gomock.Any(), dlog).Return(nil)

	id, err := svc.Submit(context.Background(), "/data/log.zip")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	waitShutdown(t, svc)
}

func TestParseService_Submit_ParseError(t *testing.T) {
	t.Parallel()

	svc, repo, parserMock := newTestParseSvc(t)

	parseErr := errors.New("broken zip")

	parserMock.EXPECT().Preflight(gomock.Any()).Return(nil)
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parserMock.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, parseErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), "broken zip").Return(nil)

	id, err := svc.Submit(context.Background(), "/bad.zip")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	waitShutdown(t, svc)
}

func TestParseService_Submit_SaveError(t *testing.T) {
	t.Parallel()

	svc, repo, parserMock := newTestParseSvc(t)

	saveErr := errors.New("save failed")

	parserMock.EXPECT().Preflight(gomock.Any()).Return(nil)
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parserMock.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, nil)
	repo.EXPECT().SaveDomainLog(gomock.Any(), gomock.Any(), gomock.Any()).Return(saveErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	_, err := svc.Submit(context.Background(), "/data/log.zip")
	require.NoError(t, err)

	waitShutdown(t, svc)
}

func TestParseService_Submit_InsertError(t *testing.T) {
	t.Parallel()

	svc, repo, parserMock := newTestParseSvc(t)

	insertErr := errors.New("db down")
	parserMock.EXPECT().Preflight(gomock.Any()).Return(nil)
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(insertErr)

	id, err := svc.Submit(context.Background(), "/data/log.zip")
	require.Error(t, err)
	assert.ErrorIs(t, err, insertErr)
	assert.Equal(t, uuid.Nil, id)
}

func TestParseService_Submit_PreflightError(t *testing.T) {
	t.Parallel()

	svc, _, parserMock := newTestParseSvc(t)

	parserMock.EXPECT().Preflight("/missing.zip").Return(parser.ErrInputNotFound)

	id, err := svc.Submit(context.Background(), "/missing.zip")
	require.Error(t, err)
	assert.ErrorIs(t, err, parser.ErrInputNotFound)
	assert.Equal(t, uuid.Nil, id)
}

func TestParseService_Submit_MarkFailedErrorIsSwallowed(t *testing.T) {
	t.Parallel()

	svc, repo, parserMock := newTestParseSvc(t)

	parseErr := errors.New("broken")
	parserMock.EXPECT().Preflight(gomock.Any()).Return(nil)
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parserMock.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, parseErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mark failed"))

	_, err := svc.Submit(context.Background(), "/bad.zip")
	require.NoError(t, err)

	waitShutdown(t, svc)
}
