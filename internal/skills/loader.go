// Package skills loads skill definitions from YAML files on the filesystem
// and upserts them into the Store on startup.
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/MackDing/hermes-manager/internal/storage"
)

// skillFile mirrors the K8s-manifest-style YAML that Helm renders.
//
//	apiVersion: hermesmanager.io/v1alpha1
//	kind: Skill
//	metadata:
//	  name: hello-skill
//	  version: "0.1.0"
//	spec:
//	  description: "..."
//	  entrypoint: "run.py"
//	  parameters: [...]
//	  required_tools: [...]
//	  required_models: [...]
type skillFile struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   skillMeta    `yaml:"metadata"`
	Spec       skillSpec    `yaml:"spec"`
}

type skillMeta struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type skillSpec struct {
	Description    string          `yaml:"description"`
	Entrypoint     string          `yaml:"entrypoint"`
	Parameters     []storage.Param `yaml:"parameters"`
	RequiredTools  []string        `yaml:"required_tools"`
	RequiredModels []string        `yaml:"required_models"`
}

// LoadFromDir reads every .yaml/.yml file under dir, parses each as a skill
// definition, and upserts it into the store. It returns the number of skills
// successfully loaded and the first error encountered (processing continues
// past individual file errors so a single bad file does not block the rest).
func LoadFromDir(ctx context.Context, store storage.Store, dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("skills: read dir %s: %w", dir, err)
	}

	var (
		loaded   int
		firstErr error
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		skill, err := parseSkillFile(path)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		if err := store.UpsertSkill(ctx, *skill); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("skills: upsert %s: %w", skill.Name, err)
			}
			continue
		}
		loaded++
	}

	return loaded, firstErr
}

// parseSkillFile reads and unmarshals a single YAML skill file.
func parseSkillFile(path string) (*storage.Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skills: read file %s: %w", path, err)
	}

	var sf skillFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("skills: parse YAML %s: %w", path, err)
	}

	if sf.Metadata.Name == "" {
		return nil, fmt.Errorf("skills: file %s has no metadata.name", path)
	}

	return &storage.Skill{
		Name:           sf.Metadata.Name,
		Version:        sf.Metadata.Version,
		Description:    sf.Spec.Description,
		Entrypoint:     sf.Spec.Entrypoint,
		Parameters:     sf.Spec.Parameters,
		RequiredTools:  sf.Spec.RequiredTools,
		RequiredModels: sf.Spec.RequiredModels,
		SourceFile:     path,
		LoadedAt:       time.Now().UTC(),
	}, nil
}
