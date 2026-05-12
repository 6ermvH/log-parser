package http

//go:generate mockgen -source=parse_handler.go -destination=mocks/parse_mock.go -package=mocks
//go:generate mockgen -source=log_handler.go -destination=mocks/log_mock.go -package=mocks
//go:generate mockgen -source=node_handler.go -destination=mocks/node_mock.go -package=mocks
//go:generate mockgen -source=ports_handler.go -destination=mocks/ports_mock.go -package=mocks
//go:generate mockgen -source=topology_handler.go -destination=mocks/topology_mock.go -package=mocks
//go:generate mockgen -source=health_handler.go -destination=mocks/health_mock.go -package=mocks
