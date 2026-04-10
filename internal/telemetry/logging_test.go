package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestMultiHandler_FansOutToAll(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})

	mh := newMultiHandler(h1, h2)
	logger := slog.New(mh)

	logger.InfoContext(context.Background(), "test message", slog.String("key", "value"))

	if !strings.Contains(buf1.String(), "test message") {
		t.Error("expected handler 1 to receive the log message")
	}
	if !strings.Contains(buf2.String(), "test message") {
		t.Error("expected handler 2 to receive the log message")
	}
}

func TestMultiHandler_Enabled_AnyEnabled(t *testing.T) {
	debug := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})
	warn := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})

	mh := newMultiHandler(debug, warn)

	// Debug should be enabled because at least one handler (debug) accepts it.
	if !mh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Enabled to return true when at least one handler accepts the level")
	}
}

func TestMultiHandler_Enabled_NoneEnabled(t *testing.T) {
	// Both handlers only accept Warn and above; Debug should be disabled.
	warn1 := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	warn2 := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})

	mh := newMultiHandler(warn1, warn2)

	if mh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Enabled to return false when no handler accepts the level")
	}
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, nil)
	mh := newMultiHandler(h)

	mh2 := mh.WithAttrs([]slog.Attr{slog.String("service", "raven")})
	logger := slog.New(mh2)
	logger.Info("with attrs")

	if !strings.Contains(buf.String(), "raven") {
		t.Error("expected WithAttrs to propagate to underlying handler")
	}
}

func TestMultiHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, nil)
	mh := newMultiHandler(h)

	mh2 := mh.WithGroup("request")
	logger := slog.New(mh2)
	logger.Info("with group", slog.String("path", "/api"))

	if !strings.Contains(buf.String(), "request") {
		t.Error("expected WithGroup to propagate to underlying handler")
	}
}
