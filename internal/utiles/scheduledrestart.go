package utiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
)

const restartScheduleFile = "restart-schedule.json"

func restartSchedulePath() string {
	return filepath.Join(configDir, restartScheduleFile)
}

func LoadRestartScheduleConfig() (*types.RestartScheduleConfig, error) {
	path := restartSchedulePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.RestartScheduleConfig{Containers: map[string]types.ContainerRestartSchedule{}}, nil
		}
		return nil, fmt.Errorf("failed to read restart schedule config: %w", err)
	}
	var cfg types.RestartScheduleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse restart schedule config: %w", err)
	}
	return &cfg, nil
}

func SaveRestartScheduleConfig(cfg *types.RestartScheduleConfig) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal restart schedule config: %w", err)
	}
	path := restartSchedulePath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write restart schedule config: %w", err)
	}
	return nil
}

func RunScheduledRestart(ctx *svc.ServiceContext) {
	cfg, err := LoadRestartScheduleConfig()
	if err != nil {
		return
	}

	dockerContainers, err := ctx.DockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return
	}

	updated := false
	for _, c := range dockerContainers {
		shortID := c.ID[:12]
		schedule, ok := cfg.Containers[shortID]
		if !ok || !schedule.Enabled {
			continue
		}

		if schedule.LastRestart == "" {
			schedule.LastRestart = time.Now().Format(time.RFC3339)
			cfg.Containers[shortID] = schedule
			updated = true
			continue
		}

		lastRestartTime, err := time.Parse(time.RFC3339, schedule.LastRestart)
		if err != nil {
			continue
		}

		elapsed := time.Since(lastRestartTime)
		interval := time.Duration(schedule.IntervalMinutes) * time.Minute
		if elapsed < interval {
			continue
		}

		err = RestartContainer(ctx, c.ID)
		if err != nil {
			continue
		}

		schedule.LastRestart = time.Now().Format(time.RFC3339)
		cfg.Containers[shortID] = schedule
		updated = true
	}

	if updated {
		SaveRestartScheduleConfig(cfg)
	}
}