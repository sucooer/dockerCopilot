package restartschedule

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLogic) List() (resp *types.Resp, err error) {
	resp = &types.Resp{}
	cfg, err := utiles.LoadRestartScheduleConfig()
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	dockerContainers, err := l.svcCtx.DockerClient.ContainerList(l.ctx, container.ListOptions{All: true})
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	var items []types.RestartScheduleItem
	for _, c := range dockerContainers {
		name := c.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		shortID := c.ID[:12]
		item := types.RestartScheduleItem{
			ContainerID:   shortID,
			ContainerName: name,
			Image:         c.Image,
			Status:        c.Status,
		}
		if s, ok := cfg.Containers[shortID]; ok {
			item.Enabled = s.Enabled
			item.IntervalMinutes = s.IntervalMinutes
			item.LastRestart = s.LastRestart
		}
		items = append(items, item)
	}

	resp.Code = 200
	resp.Msg = "success"
	resp.Data = items
	return resp, nil
}