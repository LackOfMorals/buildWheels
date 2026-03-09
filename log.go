// log.go — structured logging setup via log/slog.
package main

import (
	"log/slog"
	"os"
)

// setupLogging configures the default slog handler. All output goes to
// stderr so stdout remains clean for any scripted use.
func setupLogging(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
