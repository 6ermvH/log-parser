package logger

import (
	"log/slog"
	"os"
	"strings"
)

const (
	LevelDebug   = "debug"
	LevelInfo    = "info"
	LevelWarn    = "warn"
	LevelWarning = "warning"
	LevelError   = "error"
)

func New(level string) *slog.Logger {
	var lvl slog.Level

	switch strings.ToLower(level) {
	case LevelDebug:
		lvl = slog.LevelDebug
	case LevelWarn, LevelWarning:
		lvl = slog.LevelWarn
	case LevelError:
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
