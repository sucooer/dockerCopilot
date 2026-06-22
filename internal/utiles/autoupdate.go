package utiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

const configDir = "/data/config"
const configFile = "auto-update.json"

func autoUpdateConfigPath() string {
	return filepath.Join(configDir, configFile)
}

func LoadAutoUpdateConfig() (*types.AutoUpdateConfig, error) {
	path := autoUpdateConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.AutoUpdateConfig{Containers: map[string]types.ContainerAutoUpdate{}}, nil
		}
		return nil, fmt.Errorf("failed to read auto-update config: %w", err)
	}
	var cfg types.AutoUpdateConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse auto-update config: %w", err)
	}
	if cfg.Containers == nil {
		cfg.Containers = map[string]types.ContainerAutoUpdate{}
	}
	return &cfg, nil
}

func SaveAutoUpdateConfig(cfg *types.AutoUpdateConfig) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auto-update config: %w", err)
	}
	path := autoUpdateConfigPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write auto-update config: %w", err)
	}
	return nil
}

func RunAutoUpdateScan(ctx *svc.ServiceContext) {
	cfg, err := LoadAutoUpdateConfig()
	if err != nil {
		logx.Errorf("auto-update: failed to load config: %v", err)
		return
	}

	containerList, err := GetContainerList(ctx)
	if err != nil {
		logx.Errorf("auto-update: failed to get container list: %v", err)
		return
	}

	now := time.Now()
	changed := false

	for _, c := range containerList {
		if c.State != "running" {
			continue
		}

		shortID := c.ID[:12]
		setting, ok := cfg.Containers[shortID]
		if !ok || !setting.Enabled {
			continue
		}

		var lastCheckTime time.Time
		if setting.LastCheck != "" {
			lastCheckTime, err = time.Parse(time.RFC3339, setting.LastCheck)
			if err != nil {
				lastCheckTime = time.Time{}
			}
		}

		interval := time.Duration(setting.IntervalMinutes) * time.Minute
		if interval <= 0 {
			interval = 360 * time.Minute
		}

		if !lastCheckTime.IsZero() && now.Before(lastCheckTime.Add(interval)) {
			continue
		}

		setting.LastCheck = now.Format(time.RFC3339)

		if strings.Contains(c.Image, "0nlylty/dockercopilot") {
			cfg.Containers[shortID] = setting
			changed = true
			continue
		}

		imageList, err := GetImagesList(ctx)
		if err != nil {
			logx.Errorf("auto-update: failed to get image list: %v", err)
			cfg.Containers[shortID] = setting
			changed = true
			continue
		}

		ctx.HubImageInfo.CheckUpdate(imageList)
		checkResult, exists := ctx.HubImageInfo.Data[c.ImageID]

		needsUpdate := exists && checkResult.NeedUpdate

		if needsUpdate {
			logx.Infof("auto-update: updating %s (%s)", shortID, c.Image)
			var containerName string
			if len(c.Names) > 0 {
				containerName = strings.TrimPrefix(c.Names[0], "/")
			}
			taskID := uuid.New().String()
			err := UpdateContainer(ctx, c.ID, containerName, c.Image, true, taskID)
			if err != nil {
				logx.Errorf("auto-update: update failed for %s: %v", shortID, err)
				SendNotify(containerName, c.Image, false, err.Error())
			} else {
				setting.LastUpdate = now.Format(time.RFC3339)
				logx.Infof("auto-update: updated %s successfully", shortID)
				SendNotify(containerName, c.Image, true, "")
			}
		}

		cfg.Containers[shortID] = setting
		changed = true
	}

	if changed {
		if err := SaveAutoUpdateConfig(cfg); err != nil {
			logx.Errorf("auto-update: failed to save config: %v", err)
		}
	}
}
