package http

import (
	"log/slog"
	"net/http"
)

type Dependencies struct {
	ParseService parseSubmitter
	QueryService Services
	Pool         healthChecker
	Logger       *slog.Logger
	DataDir      string
}

type Services interface {
	logMetaQuery
	nodeQuery
	portsQuery
	topologyQuery
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler(deps.Pool, deps.Logger))
	mux.HandleFunc("POST /api/v1/parse", parseHandler(deps.ParseService, deps.Logger, deps.DataDir))
	mux.HandleFunc("GET /api/v1/topology/{log_id}", topologyHandler(deps.QueryService, deps.Logger))
	mux.HandleFunc("GET /api/v1/node/{node_id}", nodeHandler(deps.QueryService, deps.Logger))
	mux.HandleFunc("GET /api/v1/port/{node_id}", portsHandler(deps.QueryService, deps.Logger))
	mux.HandleFunc("GET /api/v1/log/{log_id}", logMetaHandler(deps.QueryService, deps.Logger))

	return recoverer(requestLogger(mux, deps.Logger), deps.Logger)
}
