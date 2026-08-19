// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package apply

import (
	"strings"
	"testing"
)

func TestApplyPatchContent_ExactMatch(t *testing.T) {
	orig := "func foo() {\n\treturn 1\n}"
	got, err := ApplyPatchContent(orig, "\treturn 1", "\treturn 2")
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
	_, err := ApplyPatchContent(orig, "\treturn 99", "\treturn 2")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "search block not found") {
		t.Fatalf("error should mention 'search block not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "func foo()") {
		t.Fatalf("error should include file context, got: %v", err)
	}
}

func TestApplyPatchContent_EmptyFile(t *testing.T) {
	got, err := ApplyPatchContent("", "something", "replacement")
	if err == nil {
		t.Fatal("expected error for empty file with non-empty search")
	}
	if got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestApplyPatchContent_EmptySearch(t *testing.T) {
	orig := "some content"
	got, err := ApplyPatchContent(orig, "", "prefix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "prefixsome content"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyPatchContent_MultipleMatches(t *testing.T) {
	orig := "a\nb\na\nc"
	got, err := ApplyPatchContent(orig, "a", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "x\nb\na\nc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}