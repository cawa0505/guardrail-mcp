// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package graphify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds the path and timeout for the Graphify MCP server binary.
type Config struct {
	BinaryPath string        `json:"binary_path"`
	Timeout    time.Duration `json:"timeout_ms"`
	Required   bool          `json:"required"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		BinaryPath: "",
		Timeout:    30 * time.Second,
		Required:   false,
	}
}

// ── Load / Save ──

func ConfigPath(stateDir string) string {
	return filepath.Join(stateDir, "graphify.json")
}

// LoadConfig reads the config from disk. If the file does not exist it
// returns DefaultConfig without error.
func LoadConfig(stateDir string) (*Config, error) {
	p := ConfigPath(stateDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read graphify config: %w", err)
	}
	// Use a raw wrapper so we can unmarshal the numeric timeout_ms into a
	// time.Duration.
	var raw struct {
		BinaryPath string `json:"binary_path"`
		TimeoutMs  int64  `json:"timeout_ms"`
		Required   bool   `json:"required"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse graphify config: %w", err)
	}
	d := DefaultConfig()
	d.BinaryPath = raw.BinaryPath
	if raw.TimeoutMs > 0 {
		d.Timeout = time.Duration(raw.TimeoutMs) * time.Millisecond
	}
	d.Required = raw.Required
	return d, nil
}

func SaveConfig(stateDir string, cfg *Config) error {
	p := ConfigPath(stateDir)
	raw := struct {
		BinaryPath string `json:"binary_path"`
		TimeoutMs  int64  `json:"timeout_ms"`
		Required   bool   `json:"required"`
	}{
		BinaryPath: cfg.BinaryPath,
		TimeoutMs:  cfg.Timeout.Milliseconds(),
		Required:   cfg.Required,
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graphify config: %w", err)
	}
	return os.WriteFile(p, data, 0644)
}