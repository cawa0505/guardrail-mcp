// Copyright 2026 jimmy. All rights reserved.
// Use of this source code is governed by a MIT-style license.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ── Language Detection ──

type LangInfo struct {
	Language string // human-readable name
	Ext      string // canonical extension (e.g. ".rs")
}

var langByExt = map[string]LangInfo{
	".rs":  {"rust", ".rs"},
	".ts":  {"typescript", ".ts"},
	".tsx": {"tsx", ".tsx"},
	".js":  {"javascript", ".js"},
	".jsx": {"javascript", ".jsx"},
	".mjs": {"javascript", ".mjs"},
	".py":  {"python", ".py"},
	".go":  {"go", ".go"},
}

var supportedExts = map[string]bool{
	".rs": true, ".ts": true, ".tsx": true,
	".js": true, ".jsx": true, ".mjs": true,
	".py": true, ".go": true,
}

func detectLang(path string) LangInfo {
	ext := strings.ToLower(filepath.Ext(path))
	if info, ok := langByExt[ext]; ok {
		return info
	}
	return LangInfo{Language: "unknown", Ext: ext}
}

func isSupported(ext string) bool {
	return supportedExts[strings.ToLower(ext)]
}

// ── Inspect Result ──

type InspectResult struct {
	Language        string `json:"language"`
	TotalLines      int    `json:"total_lines"`
	TokenReducedFrom int   `json:"token_reduced_from,omitempty"`
	TokenReducedTo   int   `json:"token_reduced_to,omitempty"`
	Content         string `json:"content"`
	Truncated       bool   `json:"truncated,omitempty"`
}

// ── Plain-Text Reducer (Regex Cleanup) ──

var (
	reSingleComment = regexp.MustCompile(`^\s*(//|#|;).*$`)
	reMLStart       = regexp.MustCompile(`/\*`)
	reMLEnd         = regexp.MustCompile(`\*/`)
	reBlank         = regexp.MustCompile(`^\s*$`)
)

const maxSkeletonLines = 100

// reduceText strips comments, blank lines, and repeated lines from text.
func reduceText(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var lines []string
	inMLComment := false
	prevLine := ""
	repeatCount := 0

	for scanner.Scan() {
		line := scanner.Text()

		// strip multi-line comments
		if inMLComment {
			if reMLEnd.MatchString(line) {
				inMLComment = false
			}
			continue
		}
		if reMLStart.MatchString(line) {
			if !reMLEnd.MatchString(line) {
				inMLComment = true
				continue
			}
			// /* ... */ on same line — strip from /* onward
			idx := reMLStart.FindStringIndex(line)
			line = strings.TrimSpace(line[:idx[0]])
		}

		// strip single-line comments (preserve shebang)
		if !strings.HasPrefix(line, "#!") && reSingleComment.MatchString(line) {
			continue
		}

		// strip blank lines
		if reBlank.MatchString(line) {
			continue
		}

		// deduplicate repeated lines (3+ consecutive identical)
		trimmed := strings.TrimSpace(line)
		if trimmed == prevLine {
			repeatCount++
			if repeatCount >= 3 {
				continue
			}
		} else {
			repeatCount = 0
		}
		prevLine = trimmed

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// countTokens is a rough token estimate (chars / 4).
func countTokens(text string) int {
	return len([]rune(text)) / 4
}

// ── Range Reader ──

func readLines(path string, start, end int) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < start {
			continue
		}
		if lineNum > end {
			break
		}
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\n"), lineNum, scanner.Err()
}

// ── Main Inspect Logic ──

func inspectFile(path, mode string, lineRange []int) (*InspectResult, error) {
	fullPath := path
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
		fullPath = filepath.Join(cwd, path)
	}

	if !isTextFile(fullPath) {
		return nil, fmt.Errorf("not a text file or file not found: %s", path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	lang := detectLang(path)
	totalLines := strings.Count(string(data), "\n")
	result := &InspectResult{
		Language:   lang.Language,
		TotalLines: totalLines,
	}

	switch mode {
	case "range":
		if len(lineRange) < 2 {
			return nil, fmt.Errorf("range mode requires line_range [start, end]")
		}
		content, _, err := readLines(fullPath, lineRange[0], lineRange[1])
		if err != nil {
			return nil, fmt.Errorf("read range: %w", err)
		}
		result.Content = content
		return result, nil

	case "full_cleaned":
		cleaned := reduceText(string(data))
		result.Content = cleaned
		result.TokenReducedFrom = countTokens(string(data))
		result.TokenReducedTo = countTokens(cleaned)
		if len(cleaned) > 80*1024 {
			result.Truncated = true
			result.Content = cleaned[:80*1024] + "\n// ... truncated (content exceeds 80KB)"
		}
		return result, nil

	default: // "skeleton"
		if isSupported(filepath.Ext(path)) {
			result.Content = skeletonByRegex(string(data), lang.Language)
		} else {
			cleaned := reduceText(string(data))
			cleanedLines := strings.Split(cleaned, "\n")
			if len(cleanedLines) > maxSkeletonLines {
				skeleton := strings.Join(cleanedLines[:maxSkeletonLines], "\n")
				skeleton += fmt.Sprintf("\n// ... skeleton truncated: %d reduced lines, showing first %d", len(cleanedLines), maxSkeletonLines)
				result.Content = skeleton
				result.Truncated = true
			} else {
				result.Content = cleaned
			}
		}
		result.TokenReducedFrom = countTokens(string(data))
		result.TokenReducedTo = countTokens(result.Content)
		return result, nil
	}
}

// skeletonByRegex builds a simple skeleton using regex heuristics.
// Interim solution until tree-sitter extractors are implemented.
func skeletonByRegex(text, lang string) string {
	lines := strings.Split(text, "\n")
	var out []string
	imports := map[string]bool{}

	var declPatterns []*regexp.Regexp

	switch lang {
	case "rust":
		declPatterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(pub\s+)?(fn|struct|enum|trait|impl|mod|type|const|static)\s+\w+`),
			regexp.MustCompile(`^\s*(pub\s+)?(async\s+)?fn\s+\w+`),
			regexp.MustCompile(`^\s*use\s+\w+`),
			regexp.MustCompile(`^\s*#!\[derive\(`),
		}
	case "go":
		declPatterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(func|type|struct|interface|const|var)\s+\w+`),
			regexp.MustCompile(`^\s*func\s+\(`),
			regexp.MustCompile(`^\s*import\s+`),
		}
	case "python":
		declPatterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(def|class|async def)\s+\w+`),
			regexp.MustCompile(`^\s*(import|from)\s+\w+`),
			regexp.MustCompile(`^\s*@\w+`),
		}
	case "typescript", "tsx", "javascript":
		declPatterns = []*regexp.Regexp{
			regexp.MustCompile(`^\s*(export\s+)?(function|class|interface|type|enum)\s+\w+`),
			regexp.MustCompile(`^\s*(export\s+)?(async\s+)?function\s+\w+`),
			regexp.MustCompile(`^\s*(import|export)\s+`),
			regexp.MustCompile(`^\s*(const|let|var)\s+\w+\s*[:=]\s*(async\s+)?\(`),
			regexp.MustCompile(`^\s*@\w+`),
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		for _, pat := range declPatterns {
			if pat.MatchString(line) {
				if strings.HasPrefix(trimmed, "use ") || strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
					if imports[trimmed] {
						break
					}
					imports[trimmed] = true
				}
				out = append(out, fmt.Sprintf("Line %d: %s", i+1, trimmed))
				break
			}
		}
		if len(out) == 0 && i < 5 {
			out = append(out, fmt.Sprintf("Line %d: %s", i+1, trimmed))
		}
	}

	if len(out) == 0 {
		return reduceText(text)
	}
	return strings.Join(out, "\n")
}

// isTextFile checks if a file looks like text (no null bytes).
func isTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	return !bytes.Contains(buf[:n], []byte{0}) && utf8.Valid(buf[:n])
}
