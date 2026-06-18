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
			expects: []string{"module ollama-telemetry", "go 1.26.0", "github.com/wailsapp/wails/v2 v2.12.0"},
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
			content := mustReadFile(t, tt.path)

			for _, expected := range tt.expects {
				if !strings.Contains(content, expected) {
					t.Fatalf("expected %s to contain %q", tt.path, expected)
				}
			}

			for _, rejected := range tt.rejects {
				if strings.Contains(content, rejected) {
					t.Fatalf("expected %s to exclude %q", tt.path, rejected)
				}
			}
		})
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

	for _, tt := range scriptCases {
		t.Run(tt.name, func(t *testing.T) {
			if manifest.Scripts[tt.name] != tt.value {
				t.Fatalf("expected script %s to be %q, got %q", tt.name, tt.value, manifest.Scripts[tt.name])
			}
		})
	}

	dependencyCases := []struct {
		name string
		deps map[string]string
	}{
		{name: "react runtime", deps: manifest.Dependencies},
		{name: "lint toolchain", deps: manifest.DevDependencies},
	}

	for _, tt := range dependencyCases {
		t.Run(tt.name, func(t *testing.T) {
			required := []string{}
			if tt.name == "react runtime" {
				required = []string{"react", "react-dom"}
			} else {
				required = []string{"eslint", "eslint-plugin-import-x", "eslint-plugin-react-doctor", "typescript", "vite"}
			}

			for _, name := range required {
				if tt.deps[name] == "" {
					t.Fatalf("expected dependency %s to be declared in %s", name, tt.name)
				}
			}
		})
	}
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
			content := mustReadFile(t, tt.path)

			for _, expected := range tt.expects {
				if !strings.Contains(content, expected) {
					t.Fatalf("expected %s to contain %q", tt.path, expected)
				}
			}

			for _, rejected := range tt.rejects {
				if strings.Contains(content, rejected) {
					t.Fatalf("expected %s to exclude %q", tt.path, rejected)
				}
			}
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
