package utiles

import (
	"context"
	"encoding/binary"
	"io"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
)

const maxLogBytes = 4 << 20

const logTruncatedMarker = "\n... 日志过大已截断 ...\n"

func GetContainerLogs(ctx *svc.ServiceContext, id string, tail int, stdout, stderr bool) (string, error) {
	options := container.LogsOptions{
		ShowStdout: stdout,
		ShowStderr: stderr,
		Timestamps: true,
		Tail:       strconv.Itoa(tail),
	}
	reader, err := ctx.DockerClient.ContainerLogs(context.Background(), id, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxLogBytes+1))
	if err != nil {
		return "", err
	}
	truncated := len(data) > maxLogBytes
	if truncated {
		data = data[:maxLogBytes]
	}

	// Docker 的 ContainerLogs 返回的是多路复用帧：
	// 每帧 8 字节头 (1 字节流类型 + 3 字节填充 + 4 字节长度) + payload。
	// 仅当同时请求 stdout 和 stderr 时才是复用格式。
	// 这里按帧头解析，剥离头部只保留日志内容。
	if stdout && stderr {
		logs := stripDockerStreamHeader(data)
		if truncated {
			logs += logTruncatedMarker
		}
		return logs, nil
	}
	if truncated {
		return string(data) + logTruncatedMarker, nil
	}
	return string(data), nil
}

func stripDockerStreamHeader(data []byte) string {
	result := make([]byte, 0, len(data))
	for i := 0; i+8 <= len(data); {
		// 校验帧头
		streamType := data[i]
		if streamType != 0 && streamType != 1 && streamType != 2 {
			// 非复用格式，直接返回全部内容
			return string(data)
		}
		size := int(binary.BigEndian.Uint32(data[i+4 : i+8]))
		if i+8+size > len(data) {
			break
		}
		result = append(result, data[i+8:i+8+size]...)
		i += 8 + size
	}
	return string(result)
}
