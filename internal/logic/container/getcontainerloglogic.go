package container

import (
	"context"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContainerLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetContainerLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContainerLogsLogic {
	return &GetContainerLogsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContainerLogsLogic) GetContainerLogs(req *types.GetContainerLogsReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	tail := req.Tail
	if tail <= 0 {
		tail = 200
	}
	if tail > 5000 {
		tail = 5000
	}
	logs, err := utiles.GetContainerLogs(l.svcCtx, req.Id, tail, req.Stdout, req.Stderr)
	if err != nil {
		resp.Code = 400
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"logs": logs,
	}
	return resp, nil
}