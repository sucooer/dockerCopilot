package utiles

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
)

func RemoveContainer(svcCtx *svc.ServiceContext, id string) error {
	ctx := context.Background()

	svcCtx.DockerClient.ContainerStop(ctx, id, container.StopOptions{})

	err := svcCtx.DockerClient.ContainerRemove(ctx, id, container.RemoveOptions{
		RemoveVolumes: false,
		Force:         true,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}