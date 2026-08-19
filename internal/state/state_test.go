// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package state

import (
	"testing"
)

func TestPhaseAllowed_Allowed(t *testing.T) {
	cases := []struct {
		phase  string
		action string
	}{
		{"INIT", "get_status"},
		{"INIT", "checkpoint"},
		{"PLANNING", "inspect_context"},
		{"PLANNING", "checkpoint"},
		{"PLANNING", "get_status"},
		{"EXECUTING", "inspect_context"},
		{"EXECUTING", "apply_patch"},
		{"EXECUTING", "checkpoint"},
		{"EXECUTING", "get_status"},
		{"VERIFYING", "inspect_context"},
		{"VERIFYING", "get_status"},
		{"PAUSED", "checkpoint"},
		{"PAUSED", "get_status"},
		{"COMPLETED", "get_status"},
	}
	for _, c := range cases {
		if !PhaseAllowed(c.phase, c.action) {
			t.Errorf("PhaseAllowed(%q, %q) should be true", c.phase, c.action)
		}
	}
}

func TestPhaseAllowed_Denied(t *testing.T) {
	cases := []struct {
		phase  string
		action string
	}{
		{"INIT", "apply_patch"},
		{"INIT", "inspect_context"},
		{"PLANNING", "apply_patch"},
		{"VERIFYING", "apply_patch"},
		{"VERIFYING", "checkpoint"},
		{"PAUSED", "apply_patch"},
		{"PAUSED", "inspect_context"},
		{"COMPLETED", "apply_patch"},
		{"COMPLETED", "checkpoint"},
		{"COMPLETED", "inspect_context"},
	}
	for _, c := range cases {
		if PhaseAllowed(c.phase, c.action) {
			t.Errorf("PhaseAllowed(%q, %q) should be false", c.phase, c.action)
		}
	}
}

func TestPhaseAllowed_UnknownPhase(t *testing.T) {
	if PhaseAllowed("UNKNOWN", "get_status") {
		t.Error("PhaseAllowed(UNKNOWN, get_status) should be false")
	}
}

func TestContainsGitignoreEntry_ExactMatch(t *testing.T) {
	data := []byte("*.log\n.opencode/\nnode_modules/\n")
	if !containsGitignoreEntry(data, ".opencode/") {
		t.Error("should find .opencode/")
	}
}

func TestContainsGitignoreEntry_TrailingSlash(t *testing.T) {
	data := []byte("*.log\n.opencode/\nnode_modules/\n")
	if !containsGitignoreEntry(data, ".opencode") {
		t.Error("should match .opencode when .opencode/ is in gitignore")
	}
}

func TestContainsGitignoreEntry_NoMatch(t *testing.T) {
	data := []byte("*.log\nnode_modules/\n")
	if containsGitignoreEntry(data, ".opencode/") {
		t.Error("should not find .opencode/")
	}
}

func TestContainsGitignoreEntry_EmptyFile(t *testing.T) {
	if containsGitignoreEntry([]byte{}, ".opencode/") {
		t.Error("empty file should not match anything")
	}
}

func TestDefaultState(t *testing.T) {
	st := DefaultState()
	if st.Phase != "INIT" {
		t.Errorf("expected INIT, got %s", st.Phase)
	}
	if st.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", st.Version)
	}
	if st.CommitToken != nil {
		t.Error("expected nil CommitToken")
	}
}

func TestNextCheckpointID(t *testing.T) {
	id := NextCheckpointID()
	if len(id) < 10 {
		t.Errorf("checkpoint ID too short: %s", id)
	}
	if id[:4] != "chk_" {
		t.Errorf("expected chk_ prefix, got %s", id[:4])
	}
}

func TestTransitionAllowed_Valid(t *testing.T) {
	cases := []struct{ from, to string }{
		{"INIT", "PLANNING"},
		{"PLANNING", "EXECUTING"},
		{"PLANNING", "PLANNING"},
		{"EXECUTING", "VERIFYING"},
		{"EXECUTING", "EXECUTING"},
		{"VERIFYING", "COMPLETED"},
		{"VERIFYING", "EXECUTING"},
		{"PAUSED", "PLANNING"},
		{"PAUSED", "EXECUTING"},
	}
	for _, c := range cases {
		if !TransitionAllowed(c.from, c.to) {
			t.Errorf("TransitionAllowed(%q, %q) should be true", c.from, c.to)
		}
	}
}

func TestTransitionAllowed_Invalid(t *testing.T) {
	cases := []struct{ from, to string }{
		{"INIT", "EXECUTING"},
		{"INIT", "COMPLETED"},
		{"PLANNING", "COMPLETED"},
		{"VERIFYING", "PLANNING"},
		{"COMPLETED", "PLANNING"},
		{"COMPLETED", "EXECUTING"},
		{"UNKNOWN", "PLANNING"},
	}
	for _, c := range cases {
		if TransitionAllowed(c.from, c.to) {
			t.Errorf("TransitionAllowed(%q, %q) should be false", c.from, c.to)
		}
	}
}

func TestCreateCheckpoint_ValidTransition(t *testing.T) {
	st := DefaultState()
	if err := CreateCheckpoint(st, "start planning", "PLANNING"); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if st.Phase != "PLANNING" {
		t.Errorf("expected PLANNING, got %s", st.Phase)
	}
	if st.ActiveGoal != "start planning" {
		t.Errorf("expected 'start planning', got %s", st.ActiveGoal)
	}
	if len(st.Checkpoints) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(st.Checkpoints))
	}
	if st.Checkpoints[0].Summary != "start planning" {
		t.Errorf("expected summary 'start planning', got %s", st.Checkpoints[0].Summary)
	}
}

func TestCreateCheckpoint_InvalidTransition(t *testing.T) {
	st := DefaultState()
	err := CreateCheckpoint(st, "skip to exec", "EXECUTING")
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
	if st.Phase != "INIT" {
		t.Errorf("phase should remain INIT after failed transition, got %s", st.Phase)
	}
}

func TestCreateCheckpoint_SamePhase(t *testing.T) {
	st := &State{
		Phase: "PLANNING",
	}
	if err := CreateCheckpoint(st, "re-planning", ""); err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if st.Phase != "PLANNING" {
		t.Errorf("expected PLANNING, got %s", st.Phase)
	}
	if len(st.Checkpoints) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(st.Checkpoints))
	}
}