// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package softguard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the top-level Soft Guard configuration file.
type Config struct {
	Verifiers []VerifierConfig `json:"verifiers"`
}

// ── Load / Save ──

func ConfigPath(stateDir string) string {
	return filepath.Join(stateDir, "softguard.json")
}

func LoadConfig(stateDir string) (*Config, error) {
	p := ConfigPath(stateDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read softguard config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse softguard config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(stateDir string, cfg *Config) error {
	p := ConfigPath(stateDir)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal softguard config: %w", err)
	}
	return os.WriteFile(p, data, 0644)
}