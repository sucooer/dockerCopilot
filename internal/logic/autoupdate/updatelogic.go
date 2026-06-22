package autoupdate

import (
	"context"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateLogic {
	return &UpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateLogic) Update(req *types.AutoUpdateUpdateReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}

	validIntervals := map[int]bool{30: true, 60: true, 360: true, 720: true, 1440: true}
	if !validIntervals[req.IntervalMinutes] {
		req.IntervalMinutes = 360
	}

	cfg, err := utiles.LoadAutoUpdateConfig()
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	cfg.Containers[req.Id] = types.ContainerAutoUpdate{
		Enabled:         req.Enabled,
		IntervalMinutes: req.IntervalMinutes,
	}

	if err := utiles.SaveAutoUpdateConfig(cfg); err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{}
	return resp, nil
}
