package compose

import (
	"context"
	"path/filepath"

	"github.com/google/uuid"
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
	taskID := uuid.New().String()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				l.Errorf("Recovered from panic in ComposeUp: %v", r)
				l.svcCtx.UpdateProgress(taskID, svc.TaskProgress{
					TaskID: taskID, Percentage: 0, Message: "部署异常",
					DetailMsg: "panic", IsDone: true,
				})
			}
		}()
		projectDir := filepath.Join(l.svcCtx.ComposeDir, req.Name)
		utiles.AsyncComposeUp(l.svcCtx, projectDir, taskID)
	}()
	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"taskID": taskID,
	}
	return resp, nil
}
