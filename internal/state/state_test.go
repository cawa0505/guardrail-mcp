// Copyright 2026 jimmy. All rights reserved.
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