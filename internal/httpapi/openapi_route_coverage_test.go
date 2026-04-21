package httpapi

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var (
	runtimeRoutePattern = regexp.MustCompile(`mux\.Handle(?:Func)?\("([A-Z]+)\s+([^"]+)"`)
	openAPIPathPattern  = regexp.MustCompile(`^\s{2}(/[^:]+):\s*$`)
	openAPIMethod       = regexp.MustCompile(`^\s{4}(get|post|put|patch|delete|options|head):\s*$`)
	pathParamPattern    = regexp.MustCompile(`\{[^}]+\}`)
)

// Internal-only routes intentionally omitted from OpenAPI.
var openAPIAllowlist = map[string]bool{
	"GET /metrics": true,
}

func TestOpenAPIRouteCoverage(t *testing.T) {
	runtimeRoutes, err := extractRuntimeRoutes()
	if err != nil {
		t.Fatalf("extract runtime routes: %v", err)
	}
	openAPIRoutes, err := extractOpenAPIRoutes()
	if err != nil {
		t.Fatalf("extract OpenAPI routes: %v", err)
	}

	var missingInOpenAPI []string
	for route := range runtimeRoutes {
		if openAPIAllowlist[route] {
			continue
		}
		if !openAPIRoutes[route] {
			missingInOpenAPI = append(missingInOpenAPI, route)
		}
	}
	slices.Sort(missingInOpenAPI)

	var missingAtRuntime []string
	for route := range openAPIRoutes {
		if !runtimeRoutes[route] {
			missingAtRuntime = append(missingAtRuntime, route)
		}
	}
	slices.Sort(missingAtRuntime)

	if len(missingInOpenAPI) > 0 || len(missingAtRuntime) > 0 {
		t.Fatalf("OpenAPI/runtime route parity mismatch\nmissing_in_openapi=%v\nmissing_at_runtime=%v\nallowlist=%v",
			missingInOpenAPI, missingAtRuntime, sortedAllowlist(),
		)
	}
}

func extractRuntimeRoutes() (map[string]bool, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		matches := runtimeRoutePattern.FindAllStringSubmatch(string(raw), -1)
		for _, m := range matches {
			method := strings.ToUpper(strings.TrimSpace(m[1]))
			path := normalizePath(m[2])
			out[method+" "+path] = true
		}
	}
	return out, nil
}

func extractOpenAPIRoutes() (map[string]bool, error) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	raw, err := os.ReadFile(filepath.Join(repoRoot, "docs", "openapi.yaml"))
	if err != nil {
		return nil, err
	}

	out := map[string]bool{}
	var currentPath string
	for _, line := range strings.Split(string(raw), "\n") {
		if m := openAPIPathPattern.FindStringSubmatch(line); len(m) == 2 {
			currentPath = normalizePath(m[1])
			continue
		}
		if currentPath == "" {
			continue
		}
		if m := openAPIMethod.FindStringSubmatch(line); len(m) == 2 {
			method := strings.ToUpper(strings.TrimSpace(m[1]))
			out[method+" "+currentPath] = true
		}
	}
	return out, nil
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = pathParamPattern.ReplaceAllString(p, "{}")
	return p
}

func sortedAllowlist() []string {
	out := make([]string, 0, len(openAPIAllowlist))
	for k := range openAPIAllowlist {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
