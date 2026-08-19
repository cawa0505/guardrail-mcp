// Copyright 2026 jimmy Yen (cawa0505). All rights reserved.
// Use of this source code is governed by a MIT-style license.
package apply

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cawa0505/guardrail-mcp/internal/state"
)

// ── Patch Application ──

// ApplyPatchContent applies a search/replace to the file content.
func ApplyPatchContent(original, search, replace string) (string, error) {
	idx := strings.Index(original, search)
	if idx < 0 {
		lines := strings.SplitN(original, "\n", 12)
		limit := len(lines)
		if limit > 11 {
			limit = 11
		}
		ctx := strings.Join(lines[:limit], "\n")
		if len(lines) > 11 {
			ctx += "\n..."
		}
		return "", fmt.Errorf("search block not found in file\n\nFile content (first lines):\n%s", ctx)
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

func FindProjectRoot(path string) string {
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
			return ""
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

func DetectCompiler(projectRoot string) *Compiler {
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

func RunCompiler(compiler *Compiler, projectRoot string) *state.CompResult {
	if compiler == nil {
		return &state.CompResult{Success: true, RawOutput: "no compiler detected, skipping validation"}
	}

	cmd := exec.Command(compiler.Command, compiler.Args...)
	cmd.Dir = projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())

	if err != nil {
		return &state.CompResult{
			Success:   false,
			RawOutput: output,
		}
	}
	return &state.CompResult{
		Success:   true,
		RawOutput: output,
	}
}

// ── Staging Buffer ──

func SetupStagingDir() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir != "" {
		stagingDir := filepath.Join(dir, "statemachine-staging")
		if err := os.MkdirAll(stagingDir, 0700); err == nil {
			return stagingDir, nil
		}
	}
	stagingDir := "/tmp/statemachine-staging"
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return "", fmt.Errorf("setup staging dir: %w", err)
	}
	return stagingDir, nil
}

func BackupFile(srcPath, stagingDir string) (string, error) {
	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return "", fmt.Errorf("backup: resolve path: %w", err)
	}
	safeName := strings.ReplaceAll(absPath, "/", "_")
	backupPath := filepath.Join(stagingDir, safeName+".bak")

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("backup: read source: %w", err)
	}
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", fmt.Errorf("backup: write backup: %w", err)
	}
	return backupPath, nil
}

func RestoreFromBackup(backupPath, targetPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("restore: read backup: %w", err)
	}
	return os.WriteFile(targetPath, data, 0644)
}

func CleanupBackup(backupPath string) {
	os.Remove(backupPath)
}

// SpawnGraphifyExtract runs `graphify extract <projectDir>` in the background.
func SpawnGraphifyExtract(projectRoot string, modifiedFile string) {
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
			return
		}

		log.Printf("[graphify] extract completed in %v", elapsed)

		st, loadErr := state.LoadState()
		if loadErr != nil {
			log.Printf("[graphify] failed to load state for ast_synced update: %v", loadErr)
			return
		}
		st.ASTSynced = true
		if saveErr := state.SaveState(st); saveErr != nil {
			log.Printf("[graphify] failed to save state with ast_synced=true: %v", saveErr)
		}
	}()
}