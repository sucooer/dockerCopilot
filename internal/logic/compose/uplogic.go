package compose

import (
	"context"
	"path/filepath"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpLogic {
	return &UpLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpLogic) Up(req *types.ComposeNameReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	projectDir := filepath.Join(l.svcCtx.ComposeDir, req.Name)
	output, err := utiles.ComposeUp(projectDir)
	if err != nil {
		resp.Code = 400
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{
			"output": output,
		}
		return resp, err
	}
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"output": output,
	}
	return resp, nil
}
