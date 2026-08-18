// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"strings"
	"testing"
)

// ── applyPatchContent ──

func TestApplyPatchContent_ExactMatch(t *testing.T) {
	orig := "func foo() {\n\treturn 1\n}"
	got, err := applyPatchContent(orig, "\treturn 1", "\treturn 2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "func foo() {\n\treturn 2\n}"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyPatchContent_NotFound(t *testing.T) {
	orig := "func foo() {\n\treturn 1\n}"
	_, err := applyPatchContent(orig, "\treturn 99", "\treturn 2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "search block not found") {
		t.Fatalf("error should mention 'search block not found', got: %v", err)
	}
	// Should include file context
	if !strings.Contains(err.Error(), "func foo()") {
		t.Fatalf("error should include file context, got: %v", err)
	}
}

func TestApplyPatchContent_EmptyFile(t *testing.T) {
	got, err := applyPatchContent("", "something", "replacement")
	if err == nil {
		t.Fatal("expected error for empty file with non-empty search")
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestApplyPatchContent_EmptySearch(t *testing.T) {
	orig := "some content"
	got, err := applyPatchContent(orig, "", "prefix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty search matches at index 0, so replace is prepended
	want := "prefixsome content"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyPatchContent_MultipleMatches(t *testing.T) {
	orig := "a\nb\na\nc"
	// Only the first occurrence should be replaced
	got, err := applyPatchContent(orig, "a", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "x\nb\na\nc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// ── phaseAllowed ──

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
		if !phaseAllowed(c.phase, c.action) {
			t.Errorf("phaseAllowed(%q, %q) should be true", c.phase, c.action)
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
		if phaseAllowed(c.phase, c.action) {
			t.Errorf("phaseAllowed(%q, %q) should be false", c.phase, c.action)
		}
	}
}

func TestPhaseAllowed_UnknownPhase(t *testing.T) {
	// Unknown phase should deny everything
	if phaseAllowed("UNKNOWN", "get_status") {
		t.Error("phaseAllowed(UNKNOWN, get_status) should be false")
	}
}

// ── containsGitignoreEntry ──

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