package main

import (
	"embed"
	"flag"
	"fmt"
	"go/types"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/onlyLTY/dockerCopilot/internal/config"
	"github.com/onlyLTY/dockerCopilot/internal/handler"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/onlyLTY/dockerCopilot/internal/utiles"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/x/errors"
	xhttp "github.com/zeromicro/x/http"
)

//go:embed front/*
var embeddedFront embed.FS

var configFile = flag.String("f", "etc/dockerCopilot.yaml", "the config file")

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

func main() {
	logDir := "./logs"
	ErrSetupLog := SetupLog(logDir)
	if ErrSetupLog != nil {
		logx.Errorf("failed to setup log: %v", ErrSetupLog)
		os.Exit(1)
	}
	logx.SetLevel(logx.InfoLevel)

	flag.Parse()
	var c config.Config
	err := conf.Load(*configFile, &c, conf.UseEnv())
	if err != nil {
		logx.Errorf("无法加载配置文件出错: %v", err)
		logx.Errorf("请检查配置文件 %s 是否存在且格式正确，环境变量引用是否已配置", *configFile)
		os.Exit(1)
	}
	if err := config.ValidateSecretKey(c.Auth.AccessSecret); err != nil {
		logx.Errorf("secretKey 配置不合法: %v", err)
		logx.Errorf("请确认secretKey设置正确，要求非纯数字且大于八位")
		os.Exit(1)
	}
	origins := c.AllowedOrigins
	if len(origins) == 0 {
		origins = defaultAllowedOrigins
	}
	server := rest.MustNewServer(c.RestConf, rest.WithCors(origins...))
	defer server.Stop()
	ctx := svc.NewServiceContext(c)

	// Ensure data directory and config exist (Auto-init)
	dataDir := ctx.ImageDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logx.Errorf("Failed to create data directory: %v", err)
	}

	if err := os.MkdirAll(ctx.ComposeDir, 0755); err != nil {
		logx.Errorf("Failed to create compose directory: %v", err)
	}

	imageLogosPath := ctx.ImageLogosPath
	if _, err := os.Stat(imageLogosPath); os.IsNotExist(err) {
		defaultConfig := []byte(`// 自定义镜像logo配置
export const customImageLogos = {
};
`)
		if err := os.WriteFile(imageLogosPath, defaultConfig, 0644); err != nil {
			logx.Errorf("Failed to create default imageLogos.js: %v", err)
		}
	}

	// 启动后异步初始化镜像检查，不阻塞 HTTP 服务
	go func() {
		list, err := utiles.GetImagesList(ctx)
		if err != nil {
			logx.Errorf("获取镜像列表出错: %v", err)
		} else {
			ctx.HubImageInfo.CheckUpdate(list)
		}
	}()

	corndanmu := cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	_, err = corndanmu.AddFunc("30 * * * *", func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("定时任务 panic 已恢复: %v", r)
			}
		}()
		list, err := utiles.GetImagesList(ctx)
		if err != nil {
			logx.Errorf("获取镜像列表出错: %v", err)
			return
		}
		ctx.HubImageInfo.CheckUpdate(list)
		utiles.RunAutoUpdateScan(ctx)
		utiles.RunScheduledRestart(ctx)
	})
	if err != nil {
		logx.Errorf("添加定时任务出错: %v", err)
		os.Exit(1)
	}
	corndanmu.Start()
	defer corndanmu.Stop()
	httpx.SetErrorHandler(func(err error) (int, any) {
		switch e := err.(type) {
		case *errors.CodeMsg:
			return http.StatusOK, xhttp.BaseResponse[types.Nil]{
				Code: e.Code,
				Msg:  e.Msg,
			}
		default:
			logx.Errorf("请求处理出错: %v", err)
			return http.StatusOK, xhttp.BaseResponse[types.Nil]{
				Code: 50000,
				Msg:  "内部错误",
			}
		}
	})
	handler.RegisterHandlers(server, ctx)
	RegisterHandlers(server, ctx.ImageDir)
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	logx.Info("程序版本" + config.Version)
	server.Start()

	// 等待优雅退出信号（自更新完成后通过 ShutdownCh 通知）
	<-ctx.ShutdownCh
	logx.Info("收到退出信号，正在停止服务...")
	server.Stop()
	corndanmu.Stop()
	logx.Close()
	os.Exit(0)
}
func RegisterHandlers(engine *rest.Server, imageDir string) {
	frontFS, err := fs.Sub(embeddedFront, "front")
	if err != nil {
		log.Fatal(err)
	}

	frontFileServer := http.StripPrefix("/manager", http.FileServer(http.FS(frontFS)))

	assetsHandler := http.FileServer(http.FS(frontFS))

	// Serve custom icons — custom handler blocks dotfiles and validates resolved path
	iconRoot := http.Dir(imageDir)
	iconHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// path.Base 防止 ../ 穿越；拒绝 dotfile
		base := path.Base(r.URL.Path)
		if base == "." || base[0] == '.' {
			http.NotFound(w, r)
			return
		}
		http.FileServer(iconRoot).ServeHTTP(w, r)
	})
	iconFileServer := http.StripPrefix("/src/config/image/", iconHandler)
	engine.AddRoutes(
		[]rest.Route{
			{
				Method: http.MethodGet,
				Path:   "/src/config/image/:file",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					iconFileServer.ServeHTTP(w, r)
				},
			},
		},
	)

	engine.AddRoutes(
		[]rest.Route{
			{
				Method: http.MethodGet,
				Path:   "/manager",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					frontFileServer.ServeHTTP(w, r)
				},
			},
			{
				Method: http.MethodGet,
				Path:   "/manager/:path",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					frontFileServer.ServeHTTP(w, r)
				},
			},
			{
				Method: http.MethodGet,
				Path:   "/manager/assets/:path",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					frontFileServer.ServeHTTP(w, r)
				},
			},
			{
				Method: http.MethodGet,
				Path:   "/assets/:path",
				Handler: func(w http.ResponseWriter, r *http.Request) {
					assetsHandler.ServeHTTP(w, r)
				},
			},
		},
	)
}

// 检查并创建日志目录
func ensureLogDirectory(logDir string) error {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return os.MkdirAll(logDir, 0755) // 创建目录并设置权限
	}
	return nil
}

// SetupLog 初始化日志设置
func SetupLog(logDir string) error {
	// 检查日志目录是否存在
	if err := ensureLogDirectory(logDir); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	logConf := logx.LogConf{
		Path:     logDir,
		Level:    "info",
		KeepDays: 7,
		Compress: true,
		Mode:     "file",
	}
	logx.MustSetup(logConf)
	logx.AddWriter(logx.NewWriter(os.Stdout))
	return nil
}
