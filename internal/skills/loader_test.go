package skills_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MackDing/hermes-manager/internal/skills"
	"github.com/MackDing/hermes-manager/internal/storage"
)

// fakeStore implements storage.Store with only UpsertSkill wired.
type fakeStore struct {
	storage.Store // embed to satisfy interface; panics on unimplemented methods
	upserted      []storage.Skill
}

func (f *fakeStore) UpsertSkill(_ context.Context, s storage.Skill) error {
	f.upserted = append(f.upserted, s)
	return nil
}

func TestLoadFromDir_SingleSkill(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: hermesmanager.io/v1alpha1
kind: Skill
metadata:
  name: test-skill
  version: "1.0.0"
spec:
  description: "A test skill"
  entrypoint: "main.py"
  parameters:
    - name: input
      type: string
      required: true
      description: "The input"
  required_tools:
    - llm
  required_models:
    - gpt-4
`
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeStore{}
	n, err := skills.LoadFromDir(context.Background(), store, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 skill loaded, got %d", n)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(store.upserted))
	}

	s := store.upserted[0]
	if s.Name != "test-skill" {
		t.Errorf("name = %q, want %q", s.Name, "test-skill")
	}
	if s.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", s.Version, "1.0.0")
	}
	if s.Description != "A test skill" {
		t.Errorf("description = %q, want %q", s.Description, "A test skill")
	}
	if s.Entrypoint != "main.py" {
		t.Errorf("entrypoint = %q, want %q", s.Entrypoint, "main.py")
	}
	if len(s.Parameters) != 1 {
		t.Fatalf("parameters len = %d, want 1", len(s.Parameters))
	}
	if s.Parameters[0].Name != "input" {
		t.Errorf("param name = %q, want %q", s.Parameters[0].Name, "input")
	}
	if !s.Parameters[0].Required {
		t.Error("param required = false, want true")
	}
	if len(s.RequiredTools) != 1 || s.RequiredTools[0] != "llm" {
		t.Errorf("required_tools = %v, want [llm]", s.RequiredTools)
	}
	if len(s.RequiredModels) != 1 || s.RequiredModels[0] != "gpt-4" {
		t.Errorf("required_models = %v, want [gpt-4]", s.RequiredModels)
	}
	if s.SourceFile == "" {
		t.Error("source_file should not be empty")
	}
	if s.LoadedAt.IsZero() {
		t.Error("loaded_at should not be zero")
	}
}

func TestLoadFromDir_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeStore{}
	n, err := skills.LoadFromDir(context.Background(), store, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 skills loaded, got %d", n)
	}
}

func TestLoadFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	store := &fakeStore{}
	n, err := skills.LoadFromDir(context.Background(), store, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 skills loaded, got %d", n)
	}
}

func TestLoadFromDir_MissingName(t *testing.T) {
	dir := t.TempDir()
	content := `apiVersion: hermesmanager.io/v1alpha1
kind: Skill
metadata:
  version: "1.0.0"
spec:
  description: "No name"
`
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeStore{}
	n, err := skills.LoadFromDir(context.Background(), store, dir)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if n != 0 {
		t.Fatalf("expected 0 skills loaded, got %d", n)
	}
}

func TestLoadFromDir_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.yaml", "b.yml"} {
		content := `apiVersion: hermesmanager.io/v1alpha1
kind: Skill
metadata:
  name: skill-` + name[:1] + `
  version: "0.1.0"
spec:
  description: "Skill ` + name[:1] + `"
  entrypoint: "run.py"
`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	store := &fakeStore{}
	n, err := skills.LoadFromDir(context.Background(), store, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 skills loaded, got %d", n)
	}
}
