package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPhase1WailsScaffoldFiles(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		expects []string
		rejects []string
	}{
		{
			name:    "go module foundation declares wails dependency",
			path:    "go.mod",
			expects: []string{"module dllm-network", "go 1.26.0", "github.com/wailsapp/wails/v2 v2.12.0"},
		},
		{
			name:    "main enables hidden startup shell",
			path:    "main.go",
			expects: []string{"StartHidden:       true", "HideWindowOnClose: true", "frontend/dist", "internal/app"},
		},
		{
			name:    "wails config uses bun workflows",
			path:    "wails.json",
			expects: []string{"\"frontend:install\": \"bun install\"", "\"frontend:build\": \"bun run build\"", "\"frontend:dev:watcher\": \"bun run dev\""},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assertFileContainsAndExcludes(t, tt.path, tt.expects, tt.rejects)
		})
	}
}

// assertFileContainsAndExcludes verifies path contains every expected substring
// and excludes every rejected substring.
func assertFileContainsAndExcludes(t *testing.T, path string, expects, rejects []string) {
	t.Helper()
	content := mustReadFile(t, path)

	for _, expected := range expects {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected %s to contain %q", path, expected)
		}
	}

	for _, rejected := range rejects {
		if strings.Contains(content, rejected) {
			t.Fatalf("expected %s to exclude %q", path, rejected)
		}
	}
}

func TestPhase1FrontendPackageScripts(t *testing.T) {
	t.Parallel()

	type packageManifest struct {
		Type            string            `json:"type"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	content := mustReadFile(t, "frontend/package.json")

	var manifest packageManifest
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		t.Fatalf("unmarshal frontend/package.json: %v", err)
	}

	if manifest.Type != "module" {
		t.Fatalf("expected package type to be module, got %q", manifest.Type)
	}

	scriptCases := []struct {
		name  string
		value string
	}{
		{name: "dev", value: "vite"},
		{name: "build", value: "tsc --noEmit && vite build"},
		{name: "lint", value: "eslint ."},
		{name: "typecheck", value: "tsc --noEmit"},
		{name: "validate", value: "bun run lint && bun run typecheck"},
		{name: "doctor:react", value: "bunx react-doctor@0.5.1 --verbose --no-telemetry"},
	}
	assertPackageScriptValues(t, manifest.Scripts, scriptCases)

	assertDependenciesDeclared(t, manifest.Dependencies, []string{"react", "react-dom"}, "react runtime")
	assertDependenciesDeclared(t, manifest.DevDependencies, []string{"eslint", "eslint-plugin-import-x", "eslint-plugin-react-doctor", "typescript", "vite"}, "lint toolchain")
}

// assertPackageScriptValues verifies each expected frontend package.json script
// name/value pair in scripts.
func assertPackageScriptValues(t *testing.T, scripts map[string]string, cases []struct {
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

// assertDependenciesDeclared verifies deps contains every name in required.
func assertDependenciesDeclared(t *testing.T, deps map[string]string, required []string, depName string) {
	t.Helper()
	t.Run(depName, func(t *testing.T) {
		for _, name := range required {
			if deps[name] == "" {
				t.Fatalf("expected dependency %s to be declared in %s", name, depName)
			}
		}
	})
}

func TestPhase1FrontendArchitectureConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		path    string
		expects []string
		rejects []string
	}{
		{
			name:    "tsconfig includes src and wails bindings",
			path:    "frontend/tsconfig.json",
			expects: []string{"\"jsx\": \"react-jsx\"", "\"moduleResolution\": \"Bundler\"", "\"wailsjs/**/*.ts\""},
		},
		{
			name:    "eslint config keeps bun import resolution and react doctor",
			path:    "frontend/eslint.config.js",
			expects: []string{"createTypeScriptImportResolver", "bun: true", "reactDoctor.configs.recommended.rules", "sonarjs/cognitive-complexity", "src/app/**/*.{ts,tsx}"},
			rejects: []string{"eslint-config-expo/flat"},
		},
		{
			name:    "architecture rules target web app layering",
			path:    "frontend/eslint/architecture-rules.js",
			expects: []string{"appDeliverySyntaxRules", "featureHookAnatomySyntaxRules", "readonlyUiPropsBoundarySyntaxRules", "src/shared"},
			rejects: []string{"@react-navigation/native", "expo-sqlite"},
		},
		{
			name:    "vite emits embeddable wails assets",
			path:    "frontend/vite.config.ts",
			expects: []string{"base: './'", "emptyOutDir: false"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assertFileContainsAndExcludes(t, tt.path, tt.expects, tt.rejects)
		})
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(content)
}
