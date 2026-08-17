// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ── Patch Application ──

// applyPatchContent applies a search/replace to the file content.
// Returns the new content on success, or an error if the search block is not found.
func applyPatchContent(original, search, replace string) (string, error) {
	idx := strings.Index(original, search)
	if idx < 0 {
		return "", fmt.Errorf("search block not found in file")
	}
	return original[:idx] + replace + original[idx+len(search):], nil
}

// ── Project Root Detection ──

var projectMarkers = []string{
	"Cargo.toml",
	"tsconfig.json",
	"go.mod",
	"package.json",
	"pyproject.toml",
	"setup.py",
	"CMakeLists.txt",
	"Makefile",
}

func findProjectRoot(path string) string {
	dir := filepath.Dir(path)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		for _, marker := range projectMarkers {
			if _, err := os.Stat(filepath.Join(absDir, marker)); err == nil {
				return absDir
			}
		}
		parent := filepath.Dir(absDir)
		if parent == absDir {
			return "" // reached filesystem root
		}
		absDir = parent
	}
}

// ── Compiler Detection & Validation ──

type Compiler struct {
	Name    string
	Command string
	Args    []string
}

func detectCompiler(projectRoot string) *Compiler {
	if _, err := os.Stat(filepath.Join(projectRoot, "Cargo.toml")); err == nil {
		return &Compiler{
			Name:    "cargo",
			Command: "cargo",
			Args:    []string{"check", "--manifest-path", filepath.Join(projectRoot, "Cargo.toml")},
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "tsconfig.json")); err == nil {
		return &Compiler{
			Name:    "tsc",
			Command: "npx",
			Args:    []string{"tsc", "--noEmit", "--project", projectRoot},
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
		return &Compiler{
			Name:    "go",
			Command: "go",
			Args:    []string{"build", "./..."},
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "package.json")); err == nil {
		// Try tsc first, fall back to eslint
		if _, err := os.Stat(filepath.Join(projectRoot, "tsconfig.json")); err == nil {
			return &Compiler{
				Name:    "tsc",
				Command: "npx",
				Args:    []string{"tsc", "--noEmit", "--project", projectRoot},
			}
		}
	}
	return nil
}

func runCompiler(compiler *Compiler, projectRoot string) *CompResult {
	// If no compiler detected, skip validation
	if compiler == nil {
		return &CompResult{Success: true, RawOutput: "no compiler detected, skipping validation"}
	}

	cmd := exec.Command(compiler.Command, compiler.Args...)
	cmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())

	if err != nil {
		return &CompResult{
			Success:   false,
			RawOutput: output,
		}
	}
	return &CompResult{
		Success:   true,
		RawOutput: output,
	}
}

// ── Graphify Integration ──

// spawnGraphifyExtract runs `graphify extract <projectDir>` in the background.
// Updates ast_synced in state.json when done.
func spawnGraphifyExtract(projectRoot string, modifiedFile string) {
	go func() {
		start := time.Now()
		log.Printf("[graphify] starting extract for %s (triggered by %s)", projectRoot, modifiedFile)

		cmd := exec.Command("graphify", "extract", projectRoot)
		cmd.Dir = projectRoot

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("[graphify] extract failed after %v: %v\n%s", elapsed, err, stderr.String())
			// Don't set ast_synced on failure — next get_status will show stale
			return
		}

		log.Printf("[graphify] extract completed in %v", elapsed)

		// Update ast_synced in state.json
		st, loadErr := loadState()
		if loadErr != nil {
			log.Printf("[graphify] failed to load state for ast_synced update: %v", loadErr)
			return
		}
		st.ASTSynced = true
		if saveErr := saveState(st); saveErr != nil {
			log.Printf("[graphify] failed to save state with ast_synced=true: %v", saveErr)
		}
	}()
}