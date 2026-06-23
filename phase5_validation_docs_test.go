package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPhase5RootValidationScripts(t *testing.T) {
	t.Parallel()

	manifest := readPackageManifest(t)

	scriptCases := []struct {
		name  string
		value string
	}{
		{name: "test:go", value: "bun run scripts/go-test-project.mjs"},
		{name: "test", value: "bun run test:go && bun run scripts/frontend-run.mjs test"},
		{name: "lint", value: "bun run scripts/frontend-run.mjs lint"},
		{name: "lint:go", value: "bun run scripts/go-lint-project.mjs"},
		{name: "typecheck", value: "bun run scripts/frontend-run.mjs typecheck"},
		{name: "validate", value: "bun run test:go && bun run lint:go && bun run scripts/frontend-run.mjs lint && bun run scripts/frontend-run.mjs typecheck && bun run scripts/frontend-run.mjs test && bun run scripts/frontend-run.mjs doctor:react"},
		{name: "doctor:react", value: "bun run scripts/frontend-run.mjs doctor:react"},
	}
	assertScriptValues(t, manifest.Scripts, scriptCases)

	assertFileContainsAll(t, "scripts/go-test-project.mjs", []string{
		"['list', './...']",
		"frontend/node_modules",
		"execFileSync(GO_PATH, ['test'",
	})

	assertFileContainsAll(t, "scripts/go-lint-project.mjs", []string{
		"execFileSync(GOLANGCI_LINT_PATH, ['run', './...']",
	})

	assertFileContainsAll(t, "scripts/frontend-run.mjs", []string{
		"process.argv[2]",
		"execFileSync(BUN_PATH, ['run', scriptName]",
		"'../frontend'",
	})
}

// packageManifest is a minimal representation of package.json for validation.
type packageManifest struct {
	Scripts map[string]string `json:"scripts"`
}

// readPackageManifest reads and parses the root package.json.
func readPackageManifest(t *testing.T) packageManifest {
	t.Helper()
	content, err := os.ReadFile("package.json")
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}

	var manifest packageManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("unmarshal package.json: %v", err)
	}
	return manifest
}

// assertScriptValues verifies each expected script name/value pair in scripts.
func assertScriptValues(t *testing.T, scripts map[string]string, cases []struct {
	name  string
	value string
}) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if scripts[tt.name] != tt.value {
				t.Fatalf("expected script %s to be %q, got %q", tt.name, tt.value, scripts[tt.name])
			}
		})
	}
}

// assertFileContainsAll verifies path contains every expected substring.
func assertFileContainsAll(t *testing.T, path string, expected []string) {
	t.Helper()
	scriptContent, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	for _, fragment := range expected {
		if !strings.Contains(string(scriptContent), fragment) {
			t.Fatalf("expected %s to contain %q", path, fragment)
		}
	}
}

func TestPhase5DocumentationCoversPassiveLimitsAndValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		expects []string
	}{
		{
			name: "readme covers quickstart and passive honesty",
			path: "README.md",
			expects: []string{
				"Passive means it",
				"does not proxy traffic",
				"bun run validate",
				"docs/mcp.md",
			},
		},
		{
			name: "passive telemetry doc lists exact unavailable fields",
			path: "docs/passive-telemetry.md",
			expects: []string{
				"Exact request latency",
				"Exact token counts",
				"Exact HTTP status codes",
				"Exact request or response payloads",
				"Exact streaming chunks",
				"Confidence",
				"Evidence",
			},
		},
		{
			name: "development doc covers root validation flow",
			path: "docs/development.md",
			expects: []string{
				"bun run test:go",
				"bun run test",
				"bun run lint",
				"bun run typecheck",
				"bun run validate",
				"frontend/node_modules",
			},
		},
		{
			name: "frontend standards doc explains adapted architecture rules",
			path: "frontend/eslint/README.md",
			expects: []string{
				"autoreas-mobile",
				"app/",
				"features/",
				"shared/",
				"readonly props",
				"public JSDoc",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assertFileContainsAll(t, tt.path, tt.expects)
		})
	}
}
