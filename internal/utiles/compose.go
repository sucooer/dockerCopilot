package utiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/container"
	dockerMsgType "github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"gopkg.in/yaml.v3"
)

var composeFileNames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

var composeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ValidateComposeName(name string) error {
	if name == "" {
		return fmt.Errorf("项目名不能为空")
	}
	if len(name) > 128 {
		return fmt.Errorf("项目名过长")
	}
	if !composeNamePattern.MatchString(name) {
		return fmt.Errorf("非法项目名，仅允许字母、数字、点、下划线和短横线，且不能以点开头")
	}
	return nil
}

func findComposeFile(dir string) (string, error) {
	for _, name := range composeFileNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no compose file found in %s", dir)
}

type ComposeProject struct {
	Name         string `json:"name"`
	DirPath      string `json:"dirPath"`
	Deployed     bool   `json:"deployed"`
	RunningCount int    `json:"runningCount"`
	HasEnv       bool   `json:"hasEnv"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]composeNetwork `yaml:"networks"`
	Volumes  map[string]composeVolume  `yaml:"volumes"`
}

type composeNetworks []string

func (n *composeNetworks) UnmarshalYAML(value *yaml.Node) error {
	var list []string
	if err := value.Decode(&list); err == nil {
		*n = list
		return nil
	}
	var mapping map[string]yaml.Node
	if err := value.Decode(&mapping); err != nil {
		return err
	}
	for name := range mapping {
		*n = append(*n, name)
	}
	return nil
}

type composeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Ports         []string          `yaml:"ports"`
	Volumes       []string          `yaml:"volumes"`
	Environment   envMap            `yaml:"environment"`
	Restart       string            `yaml:"restart"`
	Networks      composeNetworks   `yaml:"networks"`
	DependsOn     []string          `yaml:"depends_on"`
	Command       string            `yaml:"command"`
}

type composeNetwork struct {
	Driver string `yaml:"driver"`
}

type composeVolume struct {
	Driver string `yaml:"driver"`
}

type envMap map[string]string

func (e *envMap) UnmarshalYAML(value *yaml.Node) error {
	var m map[string]string
	if err := value.Decode(&m); err == nil {
		*e = m
		return nil
	}
	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	m = make(map[string]string, len(list))
	for _, item := range list {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	*e = m
	return nil
}

func ListComposeProjects(svcCtx *svc.ServiceContext) ([]ComposeProject, error) {
	composeDir := svcCtx.ComposeDir
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose directory: %w", err)
	}

	containerList, _ := svcCtx.DockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	containerNames := make(map[string]bool)
	for _, c := range containerList {
		for _, name := range c.Names {
			containerNames[strings.TrimPrefix(name, "/")] = true
		}
	}

	var projects []ComposeProject
	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(composeDir, entry.Name())
			composePath, err := findComposeFile(dir)
			if err != nil {
				continue
			}
			data, err := os.ReadFile(composePath)
			if err != nil {
				continue
			}
			var cf composeFile
			if err := yaml.Unmarshal(data, &cf); err != nil {
				continue
			}

			runningCount := 0
			for svcName, svc := range cf.Services {
				name := svc.ContainerName
				if name == "" {
					name = svcName
				}
				if containerNames[name] {
					runningCount++
				}
			}

		projects = append(projects, ComposeProject{
			Name:         entry.Name(),
			DirPath:      dir,
			Deployed:     runningCount > 0,
			RunningCount: runningCount,
			HasEnv:       fileExists(filepath.Join(dir, ".env")),
		})
		}
	}
	return projects, nil
}

func GetComposeContent(projectDir string) (string, error) {
	composeFile, err := findComposeFile(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}
	return string(data), nil
}

// GetEnvContent 读取 .env 文件内容，不存在时返回空字符串。
func GetEnvContent(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, ".env"))
	if err != nil {
		return ""
	}
	return string(data)
}

// fileExists 检查文件是否存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func CreateComposeProject(composeDir, name, content, envContent string) error {
	if err := ValidateComposeName(name); err != nil {
		return err
	}
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
	if envContent != "" {
		envFile := filepath.Join(projectDir, ".env")
		if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
			os.RemoveAll(projectDir)
			return fmt.Errorf("failed to write .env file: %w", err)
		}
	}
	return nil
}

func UpdateComposeContent(projectDir, content, envContent string) error {
	composeFile := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to update compose file: %w", err)
	}
	envFile := filepath.Join(projectDir, ".env")
	if envContent != "" {
		if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to update .env file: %w", err)
		}
	} else {
		// envContent 为空时删除 .env 文件（如果存在）
		os.Remove(envFile)
	}
	return nil
}

func DeleteComposeProject(projectDir string) error {
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("failed to delete compose project: %w", err)
	}
	return nil
}

func AsyncComposeUp(svcCtx *svc.ServiceContext, projectDir, taskID string) {
	ctx := context.Background()

	status := func(pct int, msg, detail string) {
		svcCtx.UpdateProgress(taskID, svc.TaskProgress{
			TaskID: taskID, Percentage: pct, Name: projectDir,
			Message: msg, DetailMsg: detail, IsDone: false,
		})
	}
	done := func(msg, detail string) {
		svcCtx.UpdateProgress(taskID, svc.TaskProgress{
			TaskID: taskID, Percentage: 100, Name: projectDir,
			Message: msg, DetailMsg: detail, IsDone: true,
		})
	}
	fail := func(msg, detail string) {
		svcCtx.UpdateProgress(taskID, svc.TaskProgress{
			TaskID: taskID, Percentage: 0, Name: projectDir,
			Message: msg, DetailMsg: detail, IsDone: true,
		})
	}

	status(0, "开始部署", "正在解析 compose 文件...")

	composeFilePath, err := findComposeFile(projectDir)
	if err != nil {
		fail("部署失败", fmt.Sprintf("找不到 compose 文件: %v", err))
		return
	}
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		fail("部署失败", fmt.Sprintf("读取 compose 文件失败: %v", err))
		return
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		fail("部署失败", fmt.Sprintf("解析 compose 文件失败: %v", err))
		return
	}

	// 加载 .env 文件，合并到环境变量（inline environment 优先）
	envVars := loadEnvFile(projectDir)

	status(5, "检查网络", "正在检查 Docker 网络...")

	for networkName := range cf.Networks {
		_, err := svcCtx.DockerClient.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if err != nil {
			status(8, "创建网络", fmt.Sprintf("正在创建网络 %s...", networkName))
			_, err := svcCtx.DockerClient.NetworkCreate(ctx, networkName, network.CreateOptions{
				Driver: "bridge",
			})
			if err != nil {
				fail("创建网络失败", fmt.Sprintf("网络 %s 创建失败: %v", networkName, err))
				return
			}
		}
	}

	orderedServices := orderServices(cf.Services)
	total := len(orderedServices)
	for svcIdx, svcName := range orderedServices {
		service := cf.Services[svcName]

		if service.Image == "" {
			fail("部署失败", fmt.Sprintf("服务 %s 未指定镜像", svcName))
			return
		}

		basePct := 10 + (svcIdx * 70 / total)
		pct := basePct

		status(pct, "拉取镜像", fmt.Sprintf("[%s] 正在拉取 %s ...", svcName, service.Image))

		reader, err := svcCtx.DockerClient.ImagePull(ctx, service.Image, image.PullOptions{})
		if err != nil {
			fail("拉取镜像失败", fmt.Sprintf("服务 %s 镜像拉取失败: %v", svcName, err))
			return
		}

		decoder := json.NewDecoder(reader)
		for {
			var msg dockerMsgType.JSONMessage
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF {
					break
				}
				break
			}
			if msg.Error != nil {
				fail("拉取镜像失败", fmt.Sprintf("服务 %s: %v", svcName, msg.Error))
				reader.Close()
				return
			}
			detail := fmt.Sprintf("[%s] %s", svcName, msg.Status)
			if msg.Progress != nil {
				detail += " " + msg.Progress.String()
			}
			svcCtx.UpdateProgress(taskID, svc.TaskProgress{
				TaskID: taskID, Percentage: pct, Name: projectDir,
				Message: "拉取镜像中", DetailMsg: detail, IsDone: false,
			})
		}
		reader.Close()

		pct = basePct + 5
		status(pct, "配置容器", fmt.Sprintf("[%s] 正在配置容器...", svcName))

		containerConfig := &container.Config{
			Image: service.Image,
			Env:   make([]string, 0),
		}

		if service.ContainerName != "" {
			containerConfig.Hostname = svcName
		}
		if service.Command != "" {
			containerConfig.Cmd = strings.Fields(service.Command)
		}

		// 先从 .env 合并，再用 inline environment 覆盖
		for k, v := range envVars {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range service.Environment {
			// 移除已存在的同名 .env 变量，用 inline 值覆盖
			prefix := k + "="
			filtered := containerConfig.Env[:0]
			for _, e := range containerConfig.Env {
				if !strings.HasPrefix(e, prefix) {
					filtered = append(filtered, e)
				}
			}
			containerConfig.Env = filtered
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}

		restartPolicy := container.RestartPolicyMode(strings.ToLower(service.Restart))
		if restartPolicy == "" {
			restartPolicy = container.RestartPolicyMode("no")
		}
		hostConfig := &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: restartPolicy},
		}

		for _, portStr := range service.Ports {
			// 用 Docker 官方解析器处理 "80:80"、"80:80/udp"、"80:80:udp"、"127.0.0.1:80:80" 等格式
			specs, err := nat.ParsePortSpec(portStr)
			if err != nil {
				fail("配置容器失败", fmt.Sprintf("服务 %s 端口 %s 解析失败: %v", svcName, portStr, err))
				return
			}
			if containerConfig.ExposedPorts == nil {
				containerConfig.ExposedPorts = nat.PortSet{}
			}
			if hostConfig.PortBindings == nil {
				hostConfig.PortBindings = nat.PortMap{}
			}
			for _, spec := range specs {
				containerConfig.ExposedPorts[spec.Port] = struct{}{}
				if spec.Binding.HostPort != "" || spec.Binding.HostIP != "" {
					hostConfig.PortBindings[spec.Port] = append(hostConfig.PortBindings[spec.Port], spec.Binding)
				}
			}
		}

		for _, volStr := range service.Volumes {
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				source := parts[0]
				dest := parts[1]
				if !strings.HasPrefix(source, "/") {
					source = filepath.Join(projectDir, source)
					source = strings.Replace(source, svcCtx.ComposeDir, svcCtx.ComposeDirHost, 1)
				}
				hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", source, dest))
			}
		}

		var networkingConfig *network.NetworkingConfig
		if len(service.Networks) > 0 {
			networkingConfig = &network.NetworkingConfig{
				EndpointsConfig: make(map[string]*network.EndpointSettings),
			}
			for _, netName := range service.Networks {
				networkingConfig.EndpointsConfig[netName] = &network.EndpointSettings{}
			}
		}

		containerName := service.ContainerName
		if containerName == "" {
			containerName = svcName
		}

		pct = basePct + 20
		status(pct, "创建容器", fmt.Sprintf("[%s] 正在创建容器 %s...", svcName, containerName))

		createResp, err := svcCtx.DockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
		if err != nil {
			fail("创建容器失败", fmt.Sprintf("服务 %s: %v", svcName, err))
			return
		}

		pct = basePct + 30
		status(pct, "启动容器", fmt.Sprintf("[%s] 正在启动容器 %s (%s)...", svcName, containerName, createResp.ID[:12]))

		err = svcCtx.DockerClient.ContainerStart(ctx, createResp.ID, container.StartOptions{})
		if err != nil {
			fail("启动容器失败", fmt.Sprintf("服务 %s: %v", svcName, err))
			return
		}
	}

	done("部署完成", "所有服务已成功部署")
}

func orderServices(services map[string]composeService) []string {
	ordered := make([]string, 0, len(services))
	visited := map[string]bool{}

	var dfs func(name string)
	dfs = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		svc, ok := services[name]
		if ok {
			for _, dep := range svc.DependsOn {
				dfs(dep)
			}
		}
		ordered = append(ordered, name)
	}

	for name := range services {
		dfs(name)
	}

	return ordered
}

// loadEnvFile 读取 .env 文件并解析为 KEY=VALUE 映射。
// 忽略注释行和空行，支持 # 注释。
func loadEnvFile(projectDir string) map[string]string {
	envFile := filepath.Join(projectDir, ".env")
	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil
	}
	vars := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 支持 export KEY=VALUE 语法
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// 去除引号包裹
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			vars[key] = value
		}
	}
	return vars
}
