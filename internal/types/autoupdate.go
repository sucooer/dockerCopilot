package types

type ContainerAutoUpdate struct {
	Enabled         bool   `json:"enabled"`
	IntervalMinutes int    `json:"intervalMinutes"`
	LastCheck       string `json:"lastCheck,omitempty"`
	LastUpdate      string `json:"lastUpdate,omitempty"`
}

type AutoUpdateConfig struct {
	Containers map[string]ContainerAutoUpdate `json:"containers"`
}
