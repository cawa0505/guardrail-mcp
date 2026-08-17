// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ── State Schema ──

type State struct {
	Version     string        `json:"version"`
	Phase       string        `json:"phase"`
	ActiveGoal  string        `json:"active_goal"`
	AllowedActions []string   `json:"allowed_actions"`
	StagingBuffer StagingBuf  `json:"staging_buffer"`
	Checkpoints []Checkpoint  `json:"checkpoints"`
}

type StagingBuf struct {
	HasPendingPatch  bool          `json:"has_pending_patch"`
	TargetFile       *string       `json:"target_file"`
	PatchContent     *string       `json:"patch_content"`
	LastCompilerResult *CompResult `json:"last_compiler_result,omitempty"`
}

type CompResult struct {
	Success   bool   `json:"success"`
	RawOutput string `json:"raw_output"`
}

type Checkpoint struct {
	ID            string   `json:"id"`
	Timestamp     string   `json:"timestamp"`
	Summary       string   `json:"summary"`
	ModifiedFiles []string `json:"modified_files"`
}

// ── Phase Gate ──

var phaseActions = map[string][]string{
	"INIT":      {"get_status"},
	"PLANNING":  {"inspect_context", "checkpoint", "get_status"},
	"EXECUTING": {"inspect_context", "apply_patch", "checkpoint", "get_status"},
	"VERIFYING": {"inspect_context", "get_status"},
	"PAUSED":    {"get_status"},
	"COMPLETED": {"get_status"},
}

func phaseAllowed(phase, action string) bool {
	for _, a := range phaseActions[phase] {
		if a == action {
			return true
		}
	}
	return false
}

// ── Load / Save ──

func stateDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, ".opencode")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	// ensure .opencode/ is gitignored
	if err := ensureGitignore(cwd); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureGitignore appends .opencode/ to .gitignore if not already present.
func ensureGitignore(cwd string) error {
	gp := filepath.Join(cwd, ".gitignore")
	data, err := os.ReadFile(gp)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// .gitignore doesn't exist yet — create one
		return os.WriteFile(gp, []byte("# StateMachineMcp auto-generated state\n.opencode/\n"), 0644)
	}
	if containsGitignoreEntry(data, ".opencode/") {
		return nil // already present
	}
	// append to existing .gitignore
	f, err := os.OpenFile(gp, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	// ensure trailing newline before appending
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString("# StateMachineMcp auto-generated state\n.opencode/\n")
	return err
}

func containsGitignoreEntry(data []byte, entry string) bool {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == entry || line == entry+"/" {
			return true
		}
	}
	return false
}

func statePath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func loadState() (*State, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultState(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	// ensure allowed_actions matches current phase
	st.AllowedActions = phaseActions[st.Phase]
	return &st, nil
}

func saveState(st *State) error {
	p, err := statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(p, data, 0644)
}

func defaultState() *State {
	return &State{
		Version:     "1.0.0",
		Phase:       "INIT",
		ActiveGoal:  "",
		AllowedActions: phaseActions["INIT"],
		StagingBuffer: StagingBuf{},
		Checkpoints: []Checkpoint{},
	}
}

// ── Helpers ──

func nextCheckpointID() string {
	return fmt.Sprintf("chk_%s", time.Now().UTC().Format("20060102_150405"))
}
