package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger sets up a slog handler writing to stdout + a rotating file under
// <appDataDir>/logs/app.log. Returns the logger, also installed as the default.
func InitLogger(appDataDir string) *slog.Logger {
	logsDir := filepath.Join(appDataDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	rot := &lumberjack.Logger{
		Filename:   filepath.Join(logsDir, "app.log"),
		MaxSize:    5, // MB
		MaxBackups: 5,
		MaxAge:     30, // days
		Compress:   true,
	}

	w := io.MultiWriter(os.Stdout, rot)
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
