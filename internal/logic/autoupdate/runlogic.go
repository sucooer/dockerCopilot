package autoupdate

import (
	"context"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunLogic {
	return &RunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RunLogic) Run() (resp *types.Resp, err error) {
	resp = &types.Resp{}

	go utiles.RunAutoUpdateScan(l.svcCtx)

	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"message": "Auto-update scan triggered",
	}
	return resp, nil
}
