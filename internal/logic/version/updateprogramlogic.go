package version

import (
	"context"
	"github.com/google/uuid"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"
	"github.com/zeromicro/go-zero/core/logx"
	"time"
)

type UpdateProgramLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProgramLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProgramLogic {
	return &UpdateProgramLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateProgram 异步执行程序自更新。
// 下载/校验/解压/安装耗时可长于请求超时，必须在后台执行并通过
// ProgressStore 上报进度，请求立即返回 taskID 供前端轮询。
func (l *UpdateProgramLogic) UpdateProgram() (resp *types.Resp, err error) {
	resp = &types.Resp{}
	taskID := uuid.New().String()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				l.Errorf("Recovered from panic in UpdateProgram: %v", r)
				l.svcCtx.UpdateProgress(taskID, svc.TaskProgress{
					TaskID: taskID, Percentage: 0, Message: "更新异常",
					DetailMsg: "panic", IsDone: true,
				})
			}
		}()
		report := func(percentage int, message string) {
			l.svcCtx.UpdateProgress(taskID, svc.TaskProgress{
				TaskID: taskID, Percentage: percentage, Message: message,
				DetailMsg: message, IsDone: percentage >= 100,
			})
		}
		err := utiles.UpdateProgram(l.svcCtx, report)
		if err != nil {
			l.Errorf("程序更新失败: %v", err)
			l.svcCtx.UpdateProgress(taskID, svc.TaskProgress{
				TaskID: taskID, Percentage: 0, Message: "更新失败",
				DetailMsg: err.Error(), IsDone: true,
			})
			return
		}
		// 新二进制已落位，等待 10 秒让响应/日志落盘后优雅退出，
		// 由容器重启策略拉起新版本
		time.Sleep(10 * time.Second)
		logx.Info("程序更新完成，正在退出并重启")
		logx.Close()
		// 通知 main 协程优雅退出，而非直接 os.Exit（会跳过 defer）
		l.svcCtx.ShutdownOnce.Do(func() { close(l.svcCtx.ShutdownCh) })
	}()

	resp.Code = 200
	resp.Msg = "success"
	resp.Data = map[string]interface{}{
		"taskID": taskID,
	}
	return resp, nil
}
