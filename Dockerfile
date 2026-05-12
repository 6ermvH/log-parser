FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations

RUN CGO_ENABLED=0 go build -o /out/log-parser ./cmd/server


FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/log-parser /app/log-parser
COPY configs ./configs

EXPOSE 8080

ENTRYPOINT ["/app/log-parser"]
