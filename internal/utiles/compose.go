package utiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	Name    string `json:"name"`
	DirPath string `json:"dirPath"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]composeNetwork `yaml:"networks"`
	Volumes  map[string]composeVolume  `yaml:"volumes"`
}

type composeService struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Ports         []string          `yaml:"ports"`
	Volumes       []string          `yaml:"volumes"`
	Environment   envMap            `yaml:"environment"`
	Restart       string            `yaml:"restart"`
	Networks      []string          `yaml:"networks"`
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

func ListComposeProjects(composeDir string) ([]ComposeProject, error) {
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose directory: %w", err)
	}
	var projects []ComposeProject
	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(composeDir, entry.Name())
			if _, err := findComposeFile(dir); err == nil {
				projects = append(projects, ComposeProject{
					Name:    entry.Name(),
					DirPath: dir,
				})
			}
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

func ComposeUp(svcCtx *svc.ServiceContext, projectDir string) (string, error) {
	ctx := context.Background()
	composeFilePath, err := findComposeFile(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read compose file: %w", err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return "", fmt.Errorf("failed to parse compose file: %w", err)
	}

	var output strings.Builder

	for networkName := range cf.Networks {
		_, err := svcCtx.DockerClient.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if err != nil {
			netResp, err := svcCtx.DockerClient.NetworkCreate(ctx, networkName, network.CreateOptions{
				Driver: "bridge",
			})
			if err != nil {
				return "", fmt.Errorf("failed to create network '%s': %w", networkName, err)
			}
			output.WriteString(fmt.Sprintf("Network %s created (id: %s)\n", networkName, netResp.ID))
		}
	}

	orderedServices := orderServices(cf.Services)

	for _, svcName := range orderedServices {
		svc := cf.Services[svcName]

		if svc.Image == "" {
			return "", fmt.Errorf("service '%s' has no image specified", svcName)
		}

		output.WriteString(fmt.Sprintf("Pulling image %s...\n", svc.Image))
		reader, err := svcCtx.DockerClient.ImagePull(ctx, svc.Image, image.PullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull image '%s' for service '%s': %w", svc.Image, svcName, err)
		}
		for {
			var buf [4096]byte
			_, err := reader.Read(buf[:])
			if err != nil {
				break
			}
		}
		reader.Close()
		output.WriteString(fmt.Sprintf("Image %s pulled\n", svc.Image))

		containerConfig := &container.Config{
			Image: svc.Image,
			Env:   make([]string, 0),
		}

		if svc.ContainerName != "" {
			containerConfig.Hostname = svcName
		}
		if svc.Command != "" {
			containerConfig.Cmd = strings.Fields(svc.Command)
		}

		for k, v := range svc.Environment {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}

		restartPolicy := container.RestartPolicyMode(strings.ToLower(svc.Restart))
		if restartPolicy == "" {
			restartPolicy = container.RestartPolicyMode("no")
		}
		hostConfig := &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: restartPolicy},
		}

		for _, portStr := range svc.Ports {
			parts := strings.Split(portStr, ":")
			var hostPort, containerPort string
			switch len(parts) {
			case 2:
				hostPort = parts[0]
				containerPort = parts[1]
			case 3:
				hostPort = parts[1]
				containerPort = parts[2]
			default:
				continue
			}

			portProto := nat.Port(fmt.Sprintf("%s/tcp", containerPort))
			if containerConfig.ExposedPorts == nil {
				containerConfig.ExposedPorts = nat.PortSet{}
			}
			containerConfig.ExposedPorts[portProto] = struct{}{}

			hostPortInt, _ := strconv.Atoi(hostPort)
			if hostConfig.PortBindings == nil {
				hostConfig.PortBindings = nat.PortMap{}
			}
			hostConfig.PortBindings[portProto] = []nat.PortBinding{
				{HostPort: strconv.Itoa(hostPortInt)},
			}
		}

		for _, volStr := range svc.Volumes {
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				source := parts[0]
				dest := parts[1]
				if !strings.HasPrefix(source, "/") {
					source = filepath.Join(projectDir, source)
				}
				hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", source, dest))
			}
		}

		var networkingConfig *network.NetworkingConfig
		if len(svc.Networks) > 0 {
			networkingConfig = &network.NetworkingConfig{
				EndpointsConfig: make(map[string]*network.EndpointSettings),
			}
			for _, netName := range svc.Networks {
				networkingConfig.EndpointsConfig[netName] = &network.EndpointSettings{}
			}
		}

		containerName := svc.ContainerName
		if containerName == "" {
			containerName = svcName
		}

		createResp, err := svcCtx.DockerClient.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
		if err != nil {
			return "", fmt.Errorf("failed to create container for service '%s': %w", svcName, err)
		}
		output.WriteString(fmt.Sprintf("Container %s created (id: %s)\n", containerName, createResp.ID[:12]))

		err = svcCtx.DockerClient.ContainerStart(ctx, createResp.ID, container.StartOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to start container for service '%s': %w", svcName, err)
		}
		output.WriteString(fmt.Sprintf("Container %s started\n", containerName))
	}

	return output.String(), nil
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

		for k, v := range service.Environment {
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
			parts := strings.Split(portStr, ":")
			var hostPort, containerPort string
			switch len(parts) {
			case 2:
				hostPort = parts[0]
				containerPort = parts[1]
			case 3:
				hostPort = parts[1]
				containerPort = parts[2]
			default:
				continue
			}
			portProto := nat.Port(fmt.Sprintf("%s/tcp", containerPort))
			if containerConfig.ExposedPorts == nil {
				containerConfig.ExposedPorts = nat.PortSet{}
			}
			containerConfig.ExposedPorts[portProto] = struct{}{}
			hostPortInt, _ := strconv.Atoi(hostPort)
			if hostConfig.PortBindings == nil {
				hostConfig.PortBindings = nat.PortMap{}
			}
			hostConfig.PortBindings[portProto] = []nat.PortBinding{
				{HostPort: strconv.Itoa(hostPortInt)},
			}
		}

		for _, volStr := range service.Volumes {
			parts := strings.Split(volStr, ":")
			if len(parts) >= 2 {
				source := parts[0]
				dest := parts[1]
				if !strings.HasPrefix(source, "/") {
					source = filepath.Join(projectDir, source)
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
