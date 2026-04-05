package db

import (
	"context"
	"testing"
)

func TestNewClickHouse_EmptyHost(t *testing.T) {
	conn, err := NewClickHouse(context.Background(), ClickHouseConfig{
		Host: "",
	})
	if err != nil {
		t.Fatalf("expected nil error for empty host, got %v", err)
	}
	if conn != nil {
		t.Fatal("expected nil conn for empty host")
	}
}

func TestClickHouseConfig_Fields(t *testing.T) {
	cfg := ClickHouseConfig{
		Host:     "localhost",
		Port:     9000,
		Database: "raven",
		User:     "default",
		Password: "secret",
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.Database != "raven" {
		t.Errorf("Database = %q, want %q", cfg.Database, "raven")
	}
}
