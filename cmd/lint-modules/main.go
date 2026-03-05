// Command lint-modules checks ES module metrics against bundling thresholds.
//
// It scans the JS modules directory and reports:
//   - max import chain depth (waterfall depth)
//   - total module count
//   - total JS size (non-test files)
//
// Exit 1 if any metric exceeds its threshold.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxWaterfallDepth = 4
	maxModuleCount    = 50
	maxTotalSizeBytes = 1_000_000 // 1 MB
)

var importRe = regexp.MustCompile(`(?:from|import)\s+['"]\./(?:modules/)?([^'"]+)['"]`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: lint-modules <static-dir>\n")
		os.Exit(2)
	}
	staticDir := os.Args[1]

	modulesDir := filepath.Join(staticDir, "modules")
	entryPoint := filepath.Join(staticDir, "app.js")

	// Collect non-test module files.
	moduleFiles, err := filepath.Glob(filepath.Join(modulesDir, "*.js"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob error: %v\n", err)
		os.Exit(2)
	}

	var nonTestFiles []string
	for _, f := range moduleFiles {
		if !strings.HasSuffix(f, ".test.js") {
			nonTestFiles = append(nonTestFiles, f)
		}
	}

	// Build import graph: filename -> list of imported filenames.
	graph := make(map[string][]string)
	for _, f := range nonTestFiles {
		name := filepath.Base(f)
		graph[name] = parseImports(f)
	}
	graph["app.js"] = parseImports(entryPoint)

	// Metric 1: waterfall depth (longest import chain from app.js).
	depth := maxDepth("app.js", graph)

	// Metric 2: total non-test module count (excluding app.js).
	moduleCount := len(nonTestFiles)

	// Metric 3: total JS size of non-test files (app.js + modules).
	totalSize := fileSize(entryPoint)
	for _, f := range nonTestFiles {
		totalSize += fileSize(f)
	}

	failed := false

	if depth > maxWaterfallDepth {
		fmt.Printf("FAIL: waterfall depth %d exceeds threshold %d\n", depth, maxWaterfallDepth)
		failed = true
	} else {
		fmt.Printf("ok:   waterfall depth %d (max %d)\n", depth, maxWaterfallDepth)
	}

	if moduleCount > maxModuleCount {
		fmt.Printf("FAIL: module count %d exceeds threshold %d\n", moduleCount, maxModuleCount)
		failed = true
	} else {
		fmt.Printf("ok:   module count %d (max %d)\n", moduleCount, maxModuleCount)
	}

	if totalSize > maxTotalSizeBytes {
		fmt.Printf("FAIL: total JS size %s exceeds threshold %s\n", fmtSize(totalSize), fmtSize(maxTotalSizeBytes))
		failed = true
	} else {
		fmt.Printf("ok:   total JS size %s (max %s)\n", fmtSize(totalSize), fmtSize(maxTotalSizeBytes))
	}

	if failed {
		fmt.Println("\nES module metrics exceed bundling thresholds — consider introducing a bundler.")
		os.Exit(1)
	}
}

// parseImports extracts imported module filenames from a JS file.
func parseImports(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	matches := importRe.FindAllStringSubmatch(string(data), -1)
	var deps []string
	for _, m := range matches {
		name := m[1]
		// Normalize: strip leading path, ensure .js suffix.
		name = filepath.Base(name)
		if !strings.HasSuffix(name, ".js") {
			name += ".js"
		}
		deps = append(deps, name)
	}
	return deps
}

// maxDepth computes the longest import chain from the given root using BFS.
func maxDepth(root string, graph map[string][]string) int {
	type entry struct {
		name  string
		depth int
	}
	visited := make(map[string]bool)
	queue := []entry{{root, 0}}
	visited[root] = true
	deepest := 0

	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]
		if e.depth > deepest {
			deepest = e.depth
		}
		for _, dep := range graph[e.name] {
			if !visited[dep] {
				visited[dep] = true
				queue = append(queue, entry{dep, e.depth + 1})
			}
		}
	}
	return deepest
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func fmtSize(b int64) string {
	if b >= 1_000_000 {
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	}
	return fmt.Sprintf("%d KB", b/1000)
}
