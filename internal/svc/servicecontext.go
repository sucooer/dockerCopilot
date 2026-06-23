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
	mu                         sync.Mutex
}

type TaskProgress struct {
	TaskID     string
	Percentage int
	Message    string
	Name       string
	DetailMsg  string
	IsDone     bool
}

type ProgressStoreType map[string]TaskProgress

func NewServiceContext(c config.Config) *ServiceContext {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logx.Errorf("Unable to create docker client: %s", err)
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
	return &ServiceContext{
		Config:         c,
		ComposeDir:     composeDir,
		ComposeDirHost: composeDirHost,
		HubImageInfo:   module.NewImageCheck(),
		ProgressStore:  make(ProgressStoreType),
		DockerClient:   cli,
	}
}

func (ctx *ServiceContext) UpdateProgress(taskID string, progress TaskProgress) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.ProgressStore[taskID] = progress
}

func (ctx *ServiceContext) GetProgress(taskID string) (TaskProgress, bool) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	progress, ok := ctx.ProgressStore[taskID]
	return progress, ok
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
