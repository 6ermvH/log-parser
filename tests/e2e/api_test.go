//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	httpapi "github.com/6ermvH/log-parser/internal/api/v1/http"
	"github.com/6ermvH/log-parser/internal/parser"
	"github.com/6ermvH/log-parser/internal/service"
	"github.com/6ermvH/log-parser/internal/storage/migrate"
	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
	"github.com/6ermvH/log-parser/migrations"
)

const (
	pgImage    = "postgres:16-alpine"
	pgDB       = "test"
	pgUser     = "test"
	pgPassword = "test"

	containerStartupTimeout = 30 * time.Second
)

var testServer *httptest.Server

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, pgImage,
		tcpostgres.WithDatabase(pgDB),
		tcpostgres.WithUsername(pgUser),
		tcpostgres.WithPassword(pgPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(containerStartupTimeout),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start container: %v\n", err)

		return 1
	}

	defer func() {
		if termErr := ctr.Terminate(ctx); termErr != nil {
			fmt.Fprintf(os.Stderr, "terminate container: %v\n", termErr)
		}
	}()

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "dsn: %v\n", err)

		return 1
	}

	if mErr := migrate.Run(migrations.FS, dsn); mErr != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", mErr)

		return 1
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)

		return 1
	}
	defer pool.Close()

	testDataDir, err := filepath.Abs(filepath.Join("..", "..", "testdata"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdata path: %v\n", err)

		return 1
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := pg.NewRepository(pool)
	parserSvc := parser.New()
	parseSvc := service.NewParseService(parserSvc, repo, log)
	querySvc := service.NewQueryService(repo)

	handler := httpapi.NewRouter(httpapi.Dependencies{
		ParseService: parseSvc,
		QueryService: querySvc,
		Pool:         pool,
		Logger:       log,
		DataDir:      testDataDir,
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	testServer = server

	return m.Run()
}

type parseResp struct {
	LogID string `json:"log_id"`
}

const (
	statusProcessing = "processing"
	statusOK         = "ok"
	statusFailed     = "failed"

	logPollInterval = 50 * time.Millisecond
	logPollTimeout  = 10 * time.Second
)

type logResp struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	NodesCount int    `json:"nodes_count"`
	PortsCount int    `json:"ports_count"`
	Error      string `json:"error,omitempty"`
}

type topologyResp struct {
	Nodes []struct {
		ID   int64  `json:"id"`
		GUID string `json:"guid"`
		Type string `json:"type"`
	} `json:"nodes"`
	Ports []any `json:"ports"`
}

type nodeResp struct {
	ID         int64             `json:"id"`
	GUID       string            `json:"guid"`
	Type       string            `json:"type"`
	SwitchInfo map[string]string `json:"switch_info,omitempty"`
}

type portsResp struct {
	Ports []struct {
		ID    int64 `json:"id"`
		Num   int   `json:"port_num"`
		State int   `json:"state"`
	} `json:"ports"`
}

func doParse(t *testing.T, path string) *http.Response {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"path": path})

	req, err := http.NewRequestWithContext(t.Context(),
		http.MethodPost, testServer.URL+"/api/v1/parse", bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

func postParse(t *testing.T, path string) parseResp {
	t.Helper()

	resp := doParse(t, path)
	defer resp.Body.Close()

	var out parseResp

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	t.Logf("POST /parse [%s] %d → %+v", path, resp.StatusCode, out)

	return out
}

func waitLog(t *testing.T, logID string) logResp {
	t.Helper()

	deadline := time.Now().Add(logPollTimeout)

	for time.Now().Before(deadline) {
		var meta logResp

		status := getJSON(t, "/api/v1/log/"+logID, &meta)
		if status == http.StatusOK && meta.Status != statusProcessing {
			return meta
		}

		time.Sleep(logPollInterval)
	}

	t.Fatalf("log %s did not leave processing state within %s", logID, logPollTimeout)

	return logResp{}
}

func getJSON(t *testing.T, path string, dst any) int {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testServer.URL+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(dst))
	}

	return resp.StatusCode
}

func TestE2E_ParseSampleLog(t *testing.T) {
	parsed := postParse(t, "log.zip")
	require.NotEmpty(t, parsed.LogID)

	meta := waitLog(t, parsed.LogID)
	assert.Equal(t, statusOK, meta.Status)
	assert.Equal(t, 5, meta.NodesCount)
	assert.Positive(t, meta.PortsCount)

	var topo topologyResp

	status := getJSON(t, "/api/v1/topology/"+parsed.LogID, &topo)
	require.Equal(t, http.StatusOK, status)
	assert.Len(t, topo.Nodes, 5)
	assert.NotEmpty(t, topo.Ports)

	var switchID int64

	for _, n := range topo.Nodes {
		if n.Type == "switch" {
			switchID = n.ID

			break
		}
	}

	require.NotZero(t, switchID, "no switch in topology")

	var node nodeResp

	status = getJSON(t, fmt.Sprintf("/api/v1/node/%d", switchID), &node)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "switch", node.Type)
	assert.NotNil(t, node.SwitchInfo)

	var ports portsResp

	status = getJSON(t, fmt.Sprintf("/api/v1/port/%d", switchID), &ports)
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, ports.Ports)
}

func TestE2E_NotFound(t *testing.T) {
	missingUUID := "00000000-0000-0000-0000-000000000000"

	assert.Equal(t, http.StatusNotFound, getJSON(t, "/api/v1/log/"+missingUUID, &logResp{}))
	assert.Equal(t, http.StatusNotFound, getJSON(t, "/api/v1/topology/"+missingUUID, &topologyResp{}))
	assert.Equal(t, http.StatusNotFound, getJSON(t, "/api/v1/node/9999999", &nodeResp{}))
	assert.Equal(t, http.StatusNotFound, getJSON(t, "/api/v1/port/9999999", &portsResp{}))
}

func TestE2E_ParseValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"path traversal", "../etc/passwd"},
		{"empty path", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := doParse(t, tc.path)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestE2E_ParseMissingFile(t *testing.T) {
	resp := doParse(t, "missing.zip")
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp struct {
		Error string `json:"error"`
	}

	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	assert.Equal(t, "input file not found", errResp.Error)
}

func TestE2E_LogMismatchField(t *testing.T) {
	parsed := postParse(t, "log_mismatch_field.zip")
	require.NotEmpty(t, parsed.LogID)

	meta := waitLog(t, parsed.LogID)
	assert.Equal(t, statusFailed, meta.Status)
	assert.Contains(t, meta.Error, "column count mismatch")
}

func TestE2E_LogNotEndSection(t *testing.T) {
	parsed := postParse(t, "log_not_end_section.zip")
	require.NotEmpty(t, parsed.LogID)

	meta := waitLog(t, parsed.LogID)
	assert.Equal(t, statusFailed, meta.Status)
	assert.NotEmpty(t, meta.Error)
}
