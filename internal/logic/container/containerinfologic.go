package container

import (
	"context"
	"strings"
	"time"

	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/types"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContainerInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContainerInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContainerInfoLogic {
	return &ContainerInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ContainerInfoLogic) ContainerInfo(req *types.IdReq) (resp *types.Resp, err error) {
	resp = &types.Resp{}
	list, err := utiles.GetContainerList(l.svcCtx)
	if err != nil {
		resp.Code = 500
		resp.Msg = err.Error()
		resp.Data = map[string]interface{}{}
		return resp, err
	}
	list = utiles.CheckImageUpdate(l.svcCtx, list)
	for _, v := range list {
		if v.ID != req.Id {
			continue
		}
		containerInfo := Info{
			Id:         v.ID,
			Status:     v.State,
			CreateTime: time.Unix(v.Created, 0).Format("2006-01-02 15:04:05"),
			RunningTime: v.Status,
			HaveUpdate: v.Update,
			Ports:      v.Ports,
		}
		if len(v.Names) > 0 {
			containerInfo.Name = strings.TrimPrefix(v.Names[0], "/")
		} else {
			containerInfo.Name = "get container name error"
			l.Error("get container name error" + v.ID)
		}
		image := v.Image
		if image == "" {
			image = v.ImageID
			containerInspect, inspectErr := utiles.GetContainerInspect(l.svcCtx, v.ID)
			if inspectErr != nil {
				l.Error("get image name error" + v.ID)
			} else {
				image = containerInspect.Config.Image
			}
		}
		containerInfo.UsingImage = image
		containerInfo.CreateImage = image
		resp.Msg = "success"
		resp.Data = containerInfo
		return resp, nil
	}
	resp.Code = 404
	resp.Msg = "container not found"
	resp.Data = map[string]interface{}{}
	return resp, nil
}
