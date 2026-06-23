package restartschedule

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

func (l *UpdateLogic) Update(req *types.RestartScheduleUpdateReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	cfg, err := utiles.LoadRestartScheduleConfig()
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}

	allowed := map[int]bool{10: true, 30: true, 60: true, 360: true, 720: true, 1440: true}
	if !allowed[req.IntervalMinutes] {
		resp.Code = 400
		resp.Msg = "不支持的间隔时间"
		resp.Data = map[string]interface{}{}
		return resp, nil
	}

	cfg.Containers[req.Id] = types.ContainerRestartSchedule{
		Enabled:         req.Enabled,
		IntervalMinutes: req.IntervalMinutes,
	}

	if err := utiles.SaveRestartScheduleConfig(cfg); err != nil {
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