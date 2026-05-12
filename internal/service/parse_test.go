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

func TestParseService_Submit_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parser := mocks.NewMocklogParser(ctrl)

	dlog := domain.Log{Nodes: []domain.Node{{GUID: "0xa", Type: domain.NodeTypeHost}}}

	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parser.EXPECT().Parse("/data/log.zip").Return(dlog, nil)
	repo.EXPECT().SaveDomainLog(gomock.Any(), gomock.Any(), dlog).Return(nil)

	svc := service.NewParseService(parser, repo, discardLogger())

	id, err := svc.Submit(context.Background(), "/data/log.zip")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	waitShutdown(t, svc)
}

func TestParseService_Submit_ParseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parser := mocks.NewMocklogParser(ctrl)

	parseErr := errors.New("broken zip")

	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parser.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, parseErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), "broken zip").Return(nil)

	svc := service.NewParseService(parser, repo, discardLogger())

	id, err := svc.Submit(context.Background(), "/bad.zip")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)

	waitShutdown(t, svc)
}

func TestParseService_Submit_SaveError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parser := mocks.NewMocklogParser(ctrl)

	saveErr := errors.New("save failed")

	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parser.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, nil)
	repo.EXPECT().SaveDomainLog(gomock.Any(), gomock.Any(), gomock.Any()).Return(saveErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	svc := service.NewParseService(parser, repo, discardLogger())

	_, err := svc.Submit(context.Background(), "/data/log.zip")
	require.NoError(t, err)

	waitShutdown(t, svc)
}

func TestParseService_Submit_InsertError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parser := mocks.NewMocklogParser(ctrl)

	insertErr := errors.New("db down")
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(insertErr)

	svc := service.NewParseService(parser, repo, discardLogger())

	id, err := svc.Submit(context.Background(), "/data/log.zip")
	require.Error(t, err)
	assert.ErrorIs(t, err, insertErr)
	assert.Equal(t, uuid.Nil, id)
}

func TestParseService_Submit_MarkFailedErrorIsSwallowed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockparseRepo(ctrl)
	parser := mocks.NewMocklogParser(ctrl)

	parseErr := errors.New("broken")
	repo.EXPECT().InsertProcessingLog(gomock.Any(), gomock.Any()).Return(nil)
	parser.EXPECT().Parse(gomock.Any()).Return(domain.Log{}, parseErr)
	repo.EXPECT().MarkLogFailed(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("mark failed"))

	svc := service.NewParseService(parser, repo, discardLogger())

	_, err := svc.Submit(context.Background(), "/bad.zip")
	require.NoError(t, err)

	waitShutdown(t, svc)
}
