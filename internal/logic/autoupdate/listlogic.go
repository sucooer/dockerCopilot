package autoupdate

import (
	"context"
	"strings"

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

	cfg, err := utiles.LoadAutoUpdateConfig()
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	containerList, err := utiles.GetContainerList(l.svcCtx)
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	var items []types.AutoUpdateItem
	for _, c := range containerList {
		shortID := c.ID[:12]
		var containerName string
		if len(c.Names) > 0 {
			containerName = strings.TrimPrefix(c.Names[0], "/")
		}
		setting, exists := cfg.Containers[shortID]
		item := types.AutoUpdateItem{
			ContainerID:   shortID,
			ContainerName: containerName,
			Image:         c.Image,
			Status:        c.State,
		}
		if exists {
			item.Enabled = setting.Enabled
			item.IntervalMinutes = setting.IntervalMinutes
			item.LastCheck = setting.LastCheck
			item.LastUpdate = setting.LastUpdate
		} else {
			item.IntervalMinutes = 360
		}
		items = append(items, item)
	}

	resp.Code = 200
	resp.Msg = "success"
	resp.Data = items
	return resp, nil
}
