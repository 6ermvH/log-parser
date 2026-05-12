//go:build !integration

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/6ermvH/log-parser/internal/domain"
	"github.com/6ermvH/log-parser/internal/service"
	"github.com/6ermvH/log-parser/internal/service/mocks"
	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

func TestQueryService_GetLogMeta_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	logID := uuid.New()
	uploaded := time.Now()

	repo.EXPECT().GetLog(gomock.Any(), logID).Return(pg.LogMeta{
		ID:           logID,
		Status:       "ok",
		UploadedAt:   uploaded,
		ErrorMessage: "",
	}, nil)
	repo.EXPECT().CountByLog(gomock.Any(), logID).Return(pg.Counts{Nodes: 5, Ports: 100}, nil)

	svc := service.NewQueryService(repo)

	meta, err := svc.GetLogMeta(context.Background(), logID)
	require.NoError(t, err)
	assert.Equal(t, logID, meta.ID)
	assert.Equal(t, "ok", meta.Status)
	assert.Equal(t, 5, meta.NodesCount)
	assert.Equal(t, 100, meta.PortsCount)
}

func TestQueryService_GetLogMeta_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().GetLog(gomock.Any(), gomock.Any()).Return(pg.LogMeta{}, pg.ErrNotFound)

	svc := service.NewQueryService(repo)

	_, err := svc.GetLogMeta(context.Background(), uuid.New())
	assert.ErrorIs(t, err, service.ErrNotFound)
}

func TestQueryService_GetTopology_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	logID := uuid.New()

	repo.EXPECT().GetLog(gomock.Any(), logID).Return(pg.LogMeta{ID: logID, Status: "ok"}, nil)
	repo.EXPECT().ListNodes(gomock.Any(), logID).Return([]pg.NodeRow{
		{ID: 1, LogID: logID, GUID: "0xa", Type: domain.NodeTypeHost},
	}, nil)
	repo.EXPECT().ListPortsByLog(gomock.Any(), logID).Return([]pg.PortRow{
		{ID: 10, NodeID: 1, Num: 1, State: 4},
	}, nil)

	svc := service.NewQueryService(repo)

	topo, err := svc.GetTopology(context.Background(), logID)
	require.NoError(t, err)
	require.Len(t, topo.Nodes, 1)
	require.Len(t, topo.Ports, 1)

	assert.Equal(t, "0xa", topo.Nodes[0].GUID)
}

func TestQueryService_GetTopology_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().GetLog(gomock.Any(), gomock.Any()).Return(pg.LogMeta{}, pg.ErrNotFound)

	svc := service.NewQueryService(repo)

	_, err := svc.GetTopology(context.Background(), uuid.New())
	assert.ErrorIs(t, err, service.ErrNotFound)
}

func TestQueryService_GetNodeDetails_WithInfo(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	logID := uuid.New()
	repo.EXPECT().GetNode(gomock.Any(), int64(42)).Return(pg.NodeRow{
		ID: 42, LogID: logID, GUID: "0xsw", Type: domain.NodeTypeSwitch, Desc: "SW",
	}, nil)
	repo.EXPECT().GetNodeInfo(gomock.Any(), int64(42)).Return(pg.NodeInfoRow{
		SwitchInfo: map[string]string{"k": "v"},
	}, true, nil)

	svc := service.NewQueryService(repo)

	details, err := svc.GetNodeDetails(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, "0xsw", details.GUID)
	assert.Equal(t, "v", details.SwitchInfo["k"])
}

func TestQueryService_GetNodeDetails_NoInfo(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().GetNode(gomock.Any(), int64(7)).Return(pg.NodeRow{
		ID: 7, GUID: "0xhost", Type: domain.NodeTypeHost,
	}, nil)
	repo.EXPECT().GetNodeInfo(gomock.Any(), int64(7)).Return(pg.NodeInfoRow{}, false, nil)

	svc := service.NewQueryService(repo)

	details, err := svc.GetNodeDetails(context.Background(), 7)
	require.NoError(t, err)
	assert.Nil(t, details.SwitchInfo)
	assert.Nil(t, details.SystemInfo)
}

func TestQueryService_GetNodeDetails_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().GetNode(gomock.Any(), gomock.Any()).Return(pg.NodeRow{}, pg.ErrNotFound)

	svc := service.NewQueryService(repo)

	_, err := svc.GetNodeDetails(context.Background(), 999)
	assert.ErrorIs(t, err, service.ErrNotFound)
}

func TestQueryService_ListPortsForNode_OK(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().NodeExists(gomock.Any(), int64(1)).Return(true, nil)
	repo.EXPECT().ListPortsByNode(gomock.Any(), int64(1)).Return([]pg.PortRow{
		{ID: 10, NodeID: 1, Num: 1, State: 4},
		{ID: 11, NodeID: 1, Num: 2, State: 1},
	}, nil)

	svc := service.NewQueryService(repo)

	ports, err := svc.ListPortsForNode(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, ports, 2)
	assert.Equal(t, 4, ports[0].State)
}

func TestQueryService_ListPortsForNode_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().NodeExists(gomock.Any(), int64(999)).Return(false, nil)

	svc := service.NewQueryService(repo)

	_, err := svc.ListPortsForNode(context.Background(), 999)
	assert.ErrorIs(t, err, service.ErrNotFound)
}

func TestQueryService_NodeExists_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMockqueryRepo(ctrl)

	repo.EXPECT().NodeExists(gomock.Any(), gomock.Any()).Return(false, errors.New("db error"))

	svc := service.NewQueryService(repo)

	_, err := svc.ListPortsForNode(context.Background(), 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, service.ErrNotFound)
}
