// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package softguard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_LoadSave(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		Verifiers: []VerifierConfig{
			{Name: "test", Type: "compiler", URL: "http://localhost:9999/verify", Enabled: true, Required: true},
		},
	}
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(loaded.Verifiers) != 1 {
		t.Fatalf("expected 1 verifier, got %d", len(loaded.Verifiers))
	}
	if loaded.Verifiers[0].Name != "test" {
		t.Errorf("expected name 'test', got %s", loaded.Verifiers[0].Name)
	}
}

func TestConfig_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig for missing file: %v", err)
	}
	if cfg == nil || len(cfg.Verifiers) != 0 {
		t.Fatal("expected empty config, not nil")
	}
}

func TestConfigPath(t *testing.T) {
	p := ConfigPath("/tmp/.opencode")
	want := filepath.Join("/tmp/.opencode", "softguard.json")
	if p != want {
		t.Errorf("got %s, want %s", p, want)
	}
}

func TestRunAll_NilConfig(t *testing.T) {
	results := RunAll(context.Background(), nil, VerifierInput{})
	if results != nil {
		t.Fatal("expected nil results for nil config")
	}
}

func TestRunAll_EmptyConfig(t *testing.T) {
	results := RunAll(context.Background(), &Config{}, VerifierInput{})
	if results != nil {
		t.Fatal("expected nil results for empty config")
	}
}

func TestRunAll_DisabledVerifier(t *testing.T) {
	cfg := &Config{
		Verifiers: []VerifierConfig{
			{Name: "disabled", URL: "http://localhost:9999/verify", Enabled: false},
		},
	}
	results := RunAll(context.Background(), cfg, VerifierInput{})
	if results != nil {
		t.Fatal("expected nil results when all verifiers disabled")
	}
}

func TestRunAll_EnabledVerifierBadURL(t *testing.T) {
	cfg := &Config{
		Verifiers: []VerifierConfig{
			{Name: "bad", URL: "http://localhost:1/nonexistent", Enabled: true, Required: true},
		},
	}
	results := RunAll(context.Background(), cfg, VerifierInput{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("expected failure for bad URL")
	}
	if results[0].VerifierName != "bad" {
		t.Errorf("expected name 'bad', got %s", results[0].VerifierName)
	}
}

func TestHasRequiredFailure_None(t *testing.T) {
	results := []*VerifierResult{
		{VerifierName: "a", Passed: true, Required: true},
		{VerifierName: "b", Passed: false, Required: false},
	}
	if r := HasRequiredFailure(results); r != nil {
		t.Errorf("expected nil, got %s", r.VerifierName)
	}
}

func TestHasRequiredFailure_Found(t *testing.T) {
	results := []*VerifierResult{
		{VerifierName: "a", Passed: true, Required: true},
		{VerifierName: "b", Passed: false, Required: true},
		{VerifierName: "c", Passed: false, Required: false},
	}
	r := HasRequiredFailure(results)
	if r == nil {
		t.Fatal("expected a failure result")
	}
	if r.VerifierName != "b" {
		t.Errorf("expected 'b', got %s", r.VerifierName)
	}
}

func TestConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := ConfigPath(dir)
	os.WriteFile(p, []byte("not json"), 0644)
	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}