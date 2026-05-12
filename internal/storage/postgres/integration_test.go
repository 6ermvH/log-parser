//go:build integration

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/6ermvH/log-parser/internal/domain"
	"github.com/6ermvH/log-parser/internal/storage/migrate"
	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
	"github.com/6ermvH/log-parser/migrations"
)

const (
	seedLogID  = "11111111-1111-1111-1111-111111111111"
	pgImage    = "postgres:16-alpine"
	pgDB       = "test"
	pgUser     = "test"
	pgPassword = "test"

	testMigrationsRelativePath = "../../../test_migrations"
)

func setupRepo(t *testing.T) (*pg.Repository, *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, pgImage,
		tcpostgres.WithDatabase(pgDB),
		tcpostgres.WithUsername(pgUser),
		tcpostgres.WithPassword(pgPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ctr.Terminate(ctx)
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, migrate.Run(migrations.FS, dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	require.NoError(t, applyTestMigrations(ctx, pool, testMigrationsRelativePath))

	return pg.NewRepository(pool), pool
}

func applyTestMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return err
		}
	}

	return nil
}

func TestRepository_SaveDomainLog_RoundTrip(t *testing.T) {
	ctx := context.Background()
	repo, _ := setupRepo(t)

	logID, err := uuid.NewV7()
	require.NoError(t, err)
	require.NoError(t, repo.InsertProcessingLog(ctx, logID))

	dlog := domain.Log{
		Nodes: []domain.Node{
			{
				GUID:            "0xrt_host",
				Type:            domain.NodeTypeHost,
				Desc:            "RT_HOST",
				SystemImageGUID: "0xrt_host",
				PortGUID:        "0xrt_host",
				Ports: []domain.Port{
					{
						Num:           1,
						GUID:          "0xrt_host",
						State:         4,
						PhyState:      5,
						LinkSpeedActv: 2048,
						LinkWidthActv: 2,
						LID:           1,
						Raw:           map[string]string{"sample": "value"},
					},
				},
			},
			{
				GUID:            "0xrt_switch",
				Type:            domain.NodeTypeSwitch,
				Desc:            "RT_SWITCH",
				SystemImageGUID: "0xrt_switch",
				PortGUID:        "0xrt_switch",
				Info: &domain.NodeInfo{
					SwitchInfo: map[string]string{"LinearFDBCap": "49152"},
					SystemInfo: map[string]string{"SerialNumber": "TEST1"},
				},
				Ports: []domain.Port{
					{Num: 0, State: 4, Raw: map[string]string{}},
					{Num: 1, State: 4, Raw: map[string]string{}},
				},
			},
		},
	}

	require.NoError(t, repo.SaveDomainLog(ctx, logID, dlog))

	meta, err := repo.GetLog(ctx, logID)
	require.NoError(t, err)
	assert.Equal(t, "ok", meta.Status)
	assert.Empty(t, meta.ErrorMessage)

	counts, err := repo.CountByLog(ctx, logID)
	require.NoError(t, err)
	assert.Equal(t, 2, counts.Nodes)
	assert.Equal(t, 3, counts.Ports)

	nodes, err := repo.ListNodes(ctx, logID)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	ports, err := repo.ListPortsByLog(ctx, logID)
	require.NoError(t, err)
	require.Len(t, ports, 3)

	var switchNodeID int64

	for _, n := range nodes {
		if n.Type == domain.NodeTypeSwitch {
			switchNodeID = n.ID
		}
	}

	require.NotZero(t, switchNodeID)

	info, ok, err := repo.GetNodeInfo(ctx, switchNodeID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "49152", info.SwitchInfo["LinearFDBCap"])
	assert.Equal(t, "TEST1", info.SystemInfo["SerialNumber"])
}

func TestRepository_MarkLogFailed(t *testing.T) {
	ctx := context.Background()
	repo, _ := setupRepo(t)

	logID, err := uuid.NewV7()
	require.NoError(t, err)
	require.NoError(t, repo.InsertProcessingLog(ctx, logID))
	require.NoError(t, repo.MarkLogFailed(ctx, logID, "broken zip"))

	meta, err := repo.GetLog(ctx, logID)
	require.NoError(t, err)
	assert.Equal(t, "failed", meta.Status)
	assert.Equal(t, "broken zip", meta.ErrorMessage)
}

func TestRepository_GetLog_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _ := setupRepo(t)

	missingID, err := uuid.NewV7()
	require.NoError(t, err)

	_, err = repo.GetLog(ctx, missingID)
	assert.ErrorIs(t, err, pg.ErrNotFound)
}

func TestRepository_ReapStaleProcessing(t *testing.T) {
	ctx := context.Background()
	repo, pool := setupRepo(t)

	staleID, err := uuid.NewV7()
	require.NoError(t, err)
	require.NoError(t, repo.InsertProcessingLog(ctx, staleID))

	freshID, err := uuid.NewV7()
	require.NoError(t, err)
	require.NoError(t, repo.InsertProcessingLog(ctx, freshID))

	_, err = pool.Exec(ctx,
		`UPDATE logs SET uploaded_at = now() - interval '10 minutes' WHERE id = $1`,
		staleID,
	)
	require.NoError(t, err)

	count, err := repo.ReapStaleProcessing(ctx, 5*time.Minute)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)

	stale, err := repo.GetLog(ctx, staleID)
	require.NoError(t, err)
	assert.Equal(t, "failed", stale.Status)
	assert.Contains(t, stale.ErrorMessage, "stale")

	fresh, err := repo.GetLog(ctx, freshID)
	require.NoError(t, err)
	assert.Equal(t, "processing", fresh.Status)
}

func TestRepository_ReadsFromSeed(t *testing.T) {
	ctx := context.Background()
	repo, _ := setupRepo(t)

	seedID := uuid.MustParse(seedLogID)

	meta, err := repo.GetLog(ctx, seedID)
	require.NoError(t, err)
	assert.Equal(t, "ok", meta.Status)

	nodes, err := repo.ListNodes(ctx, seedID)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	var switchNode *pg.NodeRow

	for i := range nodes {
		if nodes[i].Type == domain.NodeTypeSwitch {
			switchNode = &nodes[i]
		}
	}

	require.NotNil(t, switchNode)

	info, ok, err := repo.GetNodeInfo(ctx, switchNode.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Gorilla", info.SystemInfo["ProductName"])

	ports, err := repo.ListPortsByNode(ctx, switchNode.ID)
	require.NoError(t, err)
	require.Len(t, ports, 1)
	assert.Equal(t, 4, ports[0].State)
}
