// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package state

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
	Version        string       `json:"version"`
	Phase          string       `json:"phase"`
	ActiveGoal     string       `json:"active_goal"`
	AllowedActions []string     `json:"allowed_actions"`
	StagingBuffer  StagingBuf   `json:"staging_buffer"`
	Checkpoints    []Checkpoint `json:"checkpoints"`
	FailedAttempts int          `json:"failed_attempts"`
	ASTSynced      bool         `json:"ast_synced"`
	CommitToken    *CommitToken `json:"commit_token,omitempty"`
}

type StagingBuf struct {
	Dir                string      `json:"dir"`
	HasPendingPatch    bool        `json:"has_pending_patch"`
	TargetFile         *string     `json:"target_file"`
	PatchContent       *string     `json:"patch_content"`
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

// CommitToken represents a single-use commit authorization token.
type CommitToken struct {
	ID         string        `json:"id"`
	CreatedAt  time.Time     `json:"created_at"`
	ExpiresAt  time.Time     `json:"expires_at"`
	Bindings   TokenBindings `json:"bindings"`
	Used       bool          `json:"used"`
	ConsumedAt *time.Time    `json:"consumed_at,omitempty"`
	Revoked    bool          `json:"revoked"`
	RevokedAt  *time.Time    `json:"revoked_at,omitempty"`
}

// TokenBindings represents the constraints a token is locked to.
type TokenBindings struct {
	ProposalHash  string `json:"proposal_hash"`
	WorkspacePath string `json:"workspace_path"`
	Revision      string `json:"revision"`
}

// ── Phase Gate ──

var PhaseActions = map[string][]string{
	"INIT":      {"checkpoint", "get_status"},
	"PLANNING":  {"inspect_context", "checkpoint", "get_status"},
	"EXECUTING": {"inspect_context", "apply_patch", "checkpoint", "get_status"},
	"VERIFYING": {"inspect_context", "get_status"},
	"PAUSED":    {"checkpoint", "get_status"},
	"COMPLETED": {"get_status"},
}

// PhaseTransitions defines valid next phases per current phase.
var PhaseTransitions = map[string][]string{
	"INIT":      {"PLANNING"},
	"PLANNING":  {"EXECUTING", "PLANNING"},
	"EXECUTING": {"VERIFYING", "EXECUTING"},
	"VERIFYING": {"COMPLETED", "EXECUTING"},
	"PAUSED":    {"PLANNING", "EXECUTING"},
	"COMPLETED": {},
}

func PhaseAllowed(phase, action string) bool {
	for _, a := range PhaseActions[phase] {
		if a == action {
			return true
		}
	}
	return false
}

// TransitionAllowed checks if moving from current to next phase is valid.
func TransitionAllowed(current, next string) bool {
	allowed, ok := PhaseTransitions[current]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == next {
			return true
		}
	}
	return false
}

// CreateCheckpoint creates a checkpoint entry, transitions phase, and saves state.
// nextPhase is the target phase; if empty, stays in the current phase.
// Returns an error if the transition is invalid.
func CreateCheckpoint(st *State, summary, nextPhase string) error {
	if nextPhase == "" {
		nextPhase = st.Phase
	}
	if !TransitionAllowed(st.Phase, nextPhase) {
		allowed := PhaseTransitions[st.Phase]
		return fmt.Errorf("cannot transition from %s to %s (allowed: %v)", st.Phase, nextPhase, allowed)
	}

	cp := Checkpoint{
		ID:        NextCheckpointID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Summary:   summary,
	}
	st.Checkpoints = append(st.Checkpoints, cp)
	st.Phase = nextPhase
	st.AllowedActions = PhaseActions[nextPhase]
	st.ActiveGoal = summary

	return SaveState(st)
}

// ── Load / Save ──

func StateDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, ".opencode")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := ensureGitignore(cwd); err != nil {
		return "", err
	}
	return dir, nil
}

func ensureGitignore(cwd string) error {
	gp := filepath.Join(cwd, ".gitignore")
	data, err := os.ReadFile(gp)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(gp, []byte("# StateMachineMcp auto-generated state\n.opencode/\n"), 0644)
	}
	if containsGitignoreEntry(data, ".opencode/") {
		return nil
	}
	f, err := os.OpenFile(gp, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
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
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func LoadState() (*State, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultState(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	st.AllowedActions = PhaseActions[st.Phase]
	return &st, nil
}

func SaveState(st *State) error {
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

func DefaultState() *State {
	return &State{
		Version:        "1.0.0",
		Phase:          "INIT",
		ActiveGoal:     "",
		AllowedActions: PhaseActions["INIT"],
		StagingBuffer:  StagingBuf{},
		Checkpoints:    []Checkpoint{},
		ASTSynced:      false,
	}
}

// ── Helpers ──

func NextCheckpointID() string {
	return fmt.Sprintf("chk_%s", time.Now().UTC().Format("20060102_150405"))
}

func ModifiedFiles(st *State) []string {
	var files []string
	seen := map[string]bool{}
	for _, cp := range st.Checkpoints {
		for _, f := range cp.ModifiedFiles {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files
}

func LastCheckpointID(st *State) string {
	if len(st.Checkpoints) == 0 {
		return ""
	}
	return st.Checkpoints[len(st.Checkpoints)-1].ID
}