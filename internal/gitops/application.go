package gitops

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

type Application struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   ApplicationMeta   `yaml:"metadata"`
	Spec       ApplicationSpec   `yaml:"spec"`
}

type ApplicationMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type ApplicationSpec struct {
	Source   SourceConfig   `yaml:"source"`
	Functions []FunctionDef  `yaml:"functions"`
	Static   *StaticConfig  `yaml:"static,omitempty"`
}

type SourceConfig struct {
	RepoURL string `yaml:"repoURL"`
	Branch  string `yaml:"branch"`
	Subdir  string `yaml:"subdir,omitempty"` // Optional subdirectory to clone (sparse checkout)
}

type FunctionDef struct {
	Path      string `yaml:"path"`
	Runtime   string `yaml:"runtime"`
	Code      string `yaml:"code,omitempty"`
	CodeFile  string `yaml:"codeFile,omitempty"`
	Dockerfile string `yaml:"dockerfile,omitempty"`
	Context   string `yaml:"context,omitempty"`
}

type StaticConfig struct {
	Dir string `yaml:"dir"`
}

func ParseApplication(data []byte) (*Application, error) {
	var app Application
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}

	if app.APIVersion != "zerverless.io/v1" {
		return nil, fmt.Errorf("invalid apiVersion: %s", app.APIVersion)
	}

	if app.Kind != "Application" {
		return nil, fmt.Errorf("invalid kind: %s", app.Kind)
	}

	return &app, nil
}

