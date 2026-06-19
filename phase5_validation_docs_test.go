package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPhase5RootValidationScripts(t *testing.T) {
	t.Parallel()

	type packageManifest struct {
		Scripts map[string]string `json:"scripts"`
	}

	content, err := os.ReadFile("package.json")
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}

	var manifest packageManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("unmarshal package.json: %v", err)
	}

	testCases := []struct {
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

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if manifest.Scripts[tt.name] != tt.value {
				t.Fatalf("expected script %s to be %q, got %q", tt.name, tt.value, manifest.Scripts[tt.name])
			}
		})
	}

	scriptContent, err := os.ReadFile("scripts/go-test-project.mjs")
	if err != nil {
		t.Fatalf("read scripts/go-test-project.mjs: %v", err)
	}

	for _, expected := range []string{
		"['list', './...']",
		"frontend/node_modules",
		"execFileSync('go', ['test'",
	} {
		if !strings.Contains(string(scriptContent), expected) {
			t.Fatalf("expected go-test-project.mjs to contain %q", expected)
		}
	}

	lintScriptContent, err := os.ReadFile("scripts/go-lint-project.mjs")
	if err != nil {
		t.Fatalf("read scripts/go-lint-project.mjs: %v", err)
	}

	for _, expected := range []string{
		"execFileSync('golangci-lint', ['run', './...']",
	} {
		if !strings.Contains(string(lintScriptContent), expected) {
			t.Fatalf("expected go-lint-project.mjs to contain %q", expected)
		}
	}

	frontendRunnerContent, err := os.ReadFile("scripts/frontend-run.mjs")
	if err != nil {
		t.Fatalf("read scripts/frontend-run.mjs: %v", err)
	}

	for _, expected := range []string{
		"process.argv[2]",
		"execFileSync('bun', ['run', scriptName]",
		"'../frontend'",
	} {
		if !strings.Contains(string(frontendRunnerContent), expected) {
			t.Fatalf("expected frontend-run.mjs to contain %q", expected)
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
				"Passive-only Windows tray app",
				"bun run test",
				"bun run validate",
				"bun run doctor:react",
				"confirmed telemetry",
				"inferred activity",
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
			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("read %s: %v", tt.path, err)
			}

			text := string(content)
			for _, expected := range tt.expects {
				if !strings.Contains(text, expected) {
					t.Fatalf("expected %s to contain %q", tt.path, expected)
				}
			}
		})
	}
}
