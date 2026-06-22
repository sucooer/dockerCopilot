package utiles

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComposeProject struct {
	Name    string `json:"name"`
	DirPath string `json:"dirPath"`
}

func ListComposeProjects(composeDir string) ([]ComposeProject, error) {
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose directory: %w", err)
	}
	var projects []ComposeProject
	for _, entry := range entries {
		if entry.IsDir() {
			composeFile := filepath.Join(composeDir, entry.Name(), "docker-compose.yml")
			if _, err := os.Stat(composeFile); err == nil {
				projects = append(projects, ComposeProject{
					Name:    entry.Name(),
					DirPath: filepath.Join(composeDir, entry.Name()),
				})
			}
		}
	}
	return projects, nil
}

func GetComposeContent(projectDir string) (string, error) {
	composeFile := filepath.Join(projectDir, "docker-compose.yml")
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}
	return string(data), nil
}

func CreateComposeProject(composeDir, name, content string) error {
	projectDir := filepath.Join(composeDir, name)
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("compose project '%s' already exists", name)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	composeFile := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		os.RemoveAll(projectDir)
		return fmt.Errorf("failed to write compose file: %w", err)
	}
	return nil
}

func UpdateComposeContent(projectDir, content string) error {
	composeFile := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to update compose file: %w", err)
	}
	return nil
}

func DeleteComposeProject(projectDir string) error {
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("failed to delete compose project: %w", err)
	}
	return nil
}

func ComposeUp(projectDir string) (string, error) {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		logx.Errorf("docker compose up failed: %s, output: %s", err, string(output))
		return string(output), fmt.Errorf("docker compose up failed: %w", err)
	}
	return string(output), nil
}
