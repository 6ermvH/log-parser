package http

import (
	"log/slog"
	"net/http"
)

type Dependencies struct {
	Parser  logParser
	Storage Repository
	Pool    healthChecker
	Logger  *slog.Logger
	DataDir string
}

type Repository interface {
	parseStorage
	logStorage
	portsStorage
	nodeStorage
	topologyStorage
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler(deps.Pool, deps.Logger))
	mux.HandleFunc("POST /api/v1/parse", parseHandler(deps.Parser, deps.Storage, deps.Logger, deps.DataDir))
	mux.HandleFunc("GET /api/v1/topology/{log_id}", topologyHandler(deps.Storage, deps.Logger))
	mux.HandleFunc("GET /api/v1/node/{node_id}", nodeHandler(deps.Storage, deps.Logger))
	mux.HandleFunc("GET /api/v1/port/{node_id}", portsHandler(deps.Storage, deps.Logger))
	mux.HandleFunc("GET /api/v1/log/{log_id}", logMetaHandler(deps.Storage, deps.Logger))

	return recoverer(requestLogger(mux, deps.Logger), deps.Logger)
}
