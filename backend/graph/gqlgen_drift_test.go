package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/scruffyprodigy/joinquest/graph/generated"
)

// TestGqlgenDrift checks if the generated code is up to date with the schema files
func TestGqlgenDrift(t *testing.T) {
	// Get the project root directory
	projectRoot := findProjectRoot(t)

	// Find all GraphQL schema files
	schemaFiles := findSchemaFiles(t, projectRoot)

	// Resolver and generated outputs that must stay in sync with schema changes.
	generatedFiles := resolverGeneratedFiles(projectRoot)

	// Check if any schema file is newer than the most recently committed generated file.
	for _, schemaFile := range schemaFiles {
		schemaCommitTime := getGitCommitTime(t, schemaFile)

		var newestGeneratedCommit time.Time
		for _, generatedFile := range generatedFiles {
			if !fileExists(generatedFile) {
				t.Errorf("Generated file does not exist: %s", generatedFile)
				continue
			}

			generatedCommitTime := getGitCommitTime(t, generatedFile)
			if generatedCommitTime.After(newestGeneratedCommit) {
				newestGeneratedCommit = generatedCommitTime
			}
		}

		if schemaCommitTime.After(newestGeneratedCommit) {
			t.Errorf("Schema file %s (last committed %s) is newer than generated files (latest %s). Run 'go run github.com/99designs/gqlgen@v0.17.81 generate' to update generated code.",
				schemaFile, schemaCommitTime.Format(time.RFC3339),
				newestGeneratedCommit.Format(time.RFC3339))
		}
	}
}

// TestGqlgenGenerationWorks verifies that gqlgen can generate code without errors
func TestGqlgenGenerationWorks(t *testing.T) {
	// This test ensures that the current schema can be processed by gqlgen
	// without compilation errors. It doesn't actually run gqlgen generate,
	// but it verifies that the generated code is valid.

	resolver := &Resolver{}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))

	// If we can create the server without errors, the generated code is valid
	if srv == nil {
		t.Error("Failed to create GraphQL server - generated code may be invalid")
	}
}

// TestSchemaFilesExist verifies that all expected schema files exist
func TestSchemaFilesExist(t *testing.T) {
	projectRoot := findProjectRoot(t)
	schemaDir := filepath.Join(projectRoot, "backend/graph/schema")

	expectedFiles := []string{
		"account.graphqls",
		"auth.graphqls",
		"catalog.graphqls",
		"core.graphqls",
		"game.graphqls",
		"match.graphqls",
		"rooms.graphqls",
		"tables.graphqls",
		"users.graphqls",
		"goods.graphqls",
		"avatars.graphqls",
		"spirit_animal.graphqls",
		"parties.graphqls",
		"developer.graphqls",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(schemaDir, filename)
		if !fileExists(filePath) {
			t.Errorf("Expected schema file does not exist: %s", filePath)
		}
	}
}

// TestGeneratedFilesExist verifies that all expected generated files exist
func TestGeneratedFilesExist(t *testing.T) {
	projectRoot := findProjectRoot(t)

	for _, filePath := range resolverGeneratedFiles(projectRoot) {
		if !fileExists(filePath) {
			t.Errorf("Expected generated file does not exist: %s", filePath)
		}
	}
}

// Helper functions

func resolverGeneratedFiles(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "backend/graph/generated/generated.go"),
		filepath.Join(projectRoot, "backend/graph/model/models_gen.go"),
		filepath.Join(projectRoot, "backend/graph/core.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/auth.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/account.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/game.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/catalog.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/match.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/users.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/rooms.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/tables.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/avatars.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/spirit_animal.resolvers.go"),
		filepath.Join(projectRoot, "backend/graph/developer.resolvers.go"),
	}
}

func findProjectRoot(t *testing.T) string {
	// Start from the current directory and walk up to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if fileExists(goModPath) {
			// We found go.mod, but we need to go up one level to get the project root
			// since we're currently in the backend directory
			return filepath.Dir(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root (go.mod file)")
		}
		dir = parent
	}
}

func findSchemaFiles(t *testing.T, projectRoot string) []string {
	schemaDir := filepath.Join(projectRoot, "backend/graph/schema")

	var schemaFiles []string
	err := filepath.Walk(schemaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".graphqls" {
			schemaFiles = append(schemaFiles, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk schema directory: %v", err)
	}

	return schemaFiles
}

func getModTime(t *testing.T, filePath string) time.Time {
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file %s: %v", filePath, err)
	}
	return info.ModTime()
}

func getGitCommitTime(t *testing.T, filePath string) time.Time {
	// Get the last commit time for the file using git log
	cmd := exec.Command("git", "log", "-1", "--format=%ci", "--", filePath)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git commit time for %s: %v", filePath, err)
	}

	timeStr := strings.TrimSpace(string(output))
	if timeStr == "" {
		return getModTime(t, filePath)
	}

	commitTime, err := time.Parse("2006-01-02 15:04:05 -0700", timeStr)
	if err != nil {
		t.Fatalf("Failed to parse git commit time '%s' for %s: %v", timeStr, filePath, err)
	}

	return commitTime
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
