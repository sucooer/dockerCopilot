package types

type ContainerRestartSchedule struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
	LastRestart     string `json:"lastRestart,omitempty"`
}

type RestartScheduleConfig struct {
	Containers map[string]ContainerRestartSchedule `json:"containers"`
}

type RestartScheduleItem struct {
	ContainerID     string `json:"containerId"`
	ContainerName   string `json:"containerName"`
	Image           string `json:"image"`
	Status          string `json:"status"`
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
	LastRestart     string `json:"lastRestart,omitempty"`
}

type RestartScheduleUpdateReq struct {
	IdReq
	Enabled         bool `json:"enabled"`
	IntervalMinutes int  `json:"intervalMinutes"`
}