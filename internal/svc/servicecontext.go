package svc

import (
	"bufio"
	"github.com/docker/docker/client"
	"github.com/onlyLTY/dockerCopilot/internal/config"
	"github.com/onlyLTY/dockerCopilot/internal/module"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"os"
	"strings"
	"sync"
	"time"
)

type ServiceContext struct {
	Config                     config.Config
	CookieCheckMiddleware      rest.Middleware
	Jwtuuid                    string
	BearerTokenCheckMiddleware rest.Middleware
	JwtSecret                  string
	PortainerJwt               string
	HubImageInfo               *module.ImageUpdateData
	IndexCheckMiddleware       rest.Middleware
	ProgressStore              ProgressStoreType
	DockerClient               *client.Client
	ComposeDir                 string
	ComposeDirHost             string
	ImageDir                   string
	ImageLogosPath             string
	LoginLimiter               LoginLimiter
	RateLimiter                *RateLimiter
	ShutdownCh                 chan struct{}
	ShutdownOnce               sync.Once
	mu                         sync.Mutex
}

type TaskProgress struct {
	TaskID     string
	Percentage int
	Message    string
	Name       string
	DetailMsg  string
	IsDone     bool
	UpdatedAt  time.Time
}

type ProgressStoreType map[string]TaskProgress

const (
	maxProgressEntries = 512
	progressEntryTTL   = 24 * time.Hour
)

func NewServiceContext(c config.Config) *ServiceContext {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logx.Errorf("Unable to create docker client: %s", err)
		os.Exit(1)
	}
	composeDir := c.ComposeDir
	if composeDir == "" {
		composeDir = "/data/compose"
	}
	composeDirHost := os.Getenv("COMPOSE_DIR_HOST")
	if composeDirHost == "" {
		composeDirHost = detectComposeDirHost(composeDir)
		if composeDirHost == "" {
			logx.Infof("COMPOSE_DIR_HOST not set and mountinfo detection failed, falling back to %s", composeDir)
			composeDirHost = composeDir
		} else {
			logx.Infof("Auto-detected COMPOSE_DIR_HOST=%s from mountinfo", composeDirHost)
		}
	}
	imageDir := c.ImageDir
	if imageDir == "" {
		imageDir = "/data/config/image"
	}
	imageLogosPath := c.ImageLogosPath
	if imageLogosPath == "" {
		imageLogosPath = "/data/config/imageLogos.js"
	}
	return &ServiceContext{
		Config:         c,
		ComposeDir:     composeDir,
		ComposeDirHost: composeDirHost,
		ImageDir:       imageDir,
		ImageLogosPath: imageLogosPath,
		HubImageInfo:   module.NewImageCheck(),
		ProgressStore:  make(ProgressStoreType),
		DockerClient:   cli,
		RateLimiter:    NewRateLimiter(),
		ShutdownCh:     make(chan struct{}),
	}
}

func (ctx *ServiceContext) UpdateProgress(taskID string, progress TaskProgress) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	progress.UpdatedAt = time.Now()
	ctx.ProgressStore[taskID] = progress
	if len(ctx.ProgressStore) >= maxProgressEntries {
		ctx.pruneProgressLocked()
	}
}

func (ctx *ServiceContext) GetProgress(taskID string) (TaskProgress, bool) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	progress, ok := ctx.ProgressStore[taskID]
	return progress, ok
}

func (ctx *ServiceContext) pruneProgressLocked() {
	now := time.Now()
	for id, p := range ctx.ProgressStore {
		if now.Sub(p.UpdatedAt) > progressEntryTTL {
			delete(ctx.ProgressStore, id)
		}
	}
}

func detectComposeDirHost(composeDir string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if fields[4] == composeDir {
			return fields[3]
		}
	}
	return ""
}
