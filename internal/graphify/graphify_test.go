// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package graphify

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BinaryPath != "" {
		t.Errorf("expected empty binary path, got %q", cfg.BinaryPath)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.Timeout)
	}
	if cfg.Required {
		t.Errorf("expected Required=false by default")
	}
}

func TestLoadConfig_NotExist(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig on empty dir: %v", err)
	}
	if cfg.BinaryPath != "" {
		t.Errorf("expected empty binary, got %q", cfg.BinaryPath)
	}
}

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	data := `{"binary_path":"/usr/bin/graphify","timeout_ms":5000,"required":true}`
	if err := os.WriteFile(filepath.Join(dir, "graphify.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BinaryPath != "/usr/bin/graphify" {
		t.Errorf("expected /usr/bin/graphify, got %q", cfg.BinaryPath)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", cfg.Timeout)
	}
	if !cfg.Required {
		t.Errorf("expected Required=true")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "graphify.json"), []byte("{bad"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		BinaryPath: "/opt/bin/graphify",
		Timeout:    10 * time.Second,
		Required:   true,
	}
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.BinaryPath != "/opt/bin/graphify" {
		t.Errorf("expected /opt/bin/graphify, got %q", loaded.BinaryPath)
	}
	if loaded.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", loaded.Timeout)
	}
	if !loaded.Required {
		t.Errorf("expected Required=true")
	}
}

func TestExtractText(t *testing.T) {
	res := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "hello"},
			&mcp.TextContent{Text: "world"},
		},
	}
	got := extractText(res)
	if got != "hello\nworld" {
		t.Errorf("expected %q, got %q", "hello\nworld", got)
	}
}

func TestExtractText_Empty(t *testing.T) {
	res := &mcp.CallToolResult{Content: nil}
	got := extractText(res)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractText_SkipsNonText(t *testing.T) {
	res := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "only"},
		},
	}
	got := extractText(res)
	if got != "only" {
		t.Errorf("expected %q, got %q", "only", got)
	}
}

func TestSkeletonExtract_NoBinary(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClient(cfg)
	_, err := c.SkeletonExtract(t.Context(), "test.go")
	if err == nil {
		t.Fatal("expected error when binary not configured")
	}
}