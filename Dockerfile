FROM golang:1.25-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -o /out/log-parser ./cmd/server


FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/log-parser /app/log-parser
COPY configs ./configs
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/app/log-parser"]
