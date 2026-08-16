package utiles

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"github.com/onlyLTY/dockerCopilot/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	updateDownloadTimeout = 5 * time.Minute
	maxExtractBytes       = 1 << 30
)

var updateHTTPClient = &http.Client{Timeout: updateDownloadTimeout}

// UpdateProgressFunc 更新过程进度回调
type UpdateProgressFunc func(percentage int, message string)

func UpdateProgram(ctx *svc.ServiceContext, progress UpdateProgressFunc) error {
	if progress != nil {
		progress(5, "开始检查最新版本")
	}
	githubProxy := os.Getenv("githubProxy")
	if githubProxy != "" {
		githubProxy = strings.TrimRight(githubProxy, "/") + "/"
	}
	versionURL := githubProxy + "https://raw.githubusercontent.com/onlyLTY/dockerCopilot/UGREEN/version"
	releaseBaseURL := githubProxy + "https://github.com/onlyLTY/dockerCopilot/releases/download"
	logx.Infof("versionURL: %s", versionURL)
	resp, err := updateHTTPClient.Get(versionURL)
	if err != nil {
		logx.Info("没有获取到最新版本信息:", err)
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("关闭resp.Body失败:", err)
		}
	}(resp.Body)

	versionData, err := io.ReadAll(resp.Body)
	logx.Infof("versionData: %s", versionData)
	if err != nil {
		logx.Info("没有获取到最新版本信息:", err)
		return nil
	}

	version := strings.TrimSpace(string(versionData))
	logx.Info("获取到最新版本：", version)
	// 2. 构造下载链接
	downloadURL := fmt.Sprintf("%s/%s/dockerCopilot-%s.tar.gz", releaseBaseURL, version, runtime.GOARCH)
	logx.Info("下载链接：", downloadURL)
	dest := "dockerCopilot.tar.gz"
	// 无论成败都清理下载和解压产物，避免多次更新后累积占用空间
	defer func() {
		_ = os.Remove(dest)
		_ = os.RemoveAll("dist")
	}()

	if progress != nil {
		progress(20, "开始下载新版本")
	}
	if err := downloadFile(downloadURL, dest); err != nil {
		logx.Error("下载失败:", err)
		return err
	}
	logx.Info("下载成功")

	if progress != nil {
		progress(60, "校验文件完整性")
	}
	checksumURL := fmt.Sprintf("%s/%s/checksums-%s.txt", releaseBaseURL, version, runtime.GOARCH)
	if err := verifyChecksum(dest, checksumURL); err != nil {
		logx.Error("校验失败:", err)
		return err
	}

	if progress != nil {
		progress(80, "解压新版本")
	}
	if err := decompressTarGz(dest, "."); err != nil {
		logx.Info("解压缩失败:", err)
		return err
	}
	logx.Info("解压缩成功")

	if progress != nil {
		progress(90, "安装新版本二进制")
	}
	// 将新版本二进制落位为可执行文件，等待进程退出后由容器重启策略拉起新版本
	newBinary := filepath.Join("dist", "linux", runtime.GOARCH, "dockerCopilot-new")
	if err := installNewBinary(newBinary); err != nil {
		logx.Error("安装新版本二进制失败:", err)
		return err
	}
	logx.Info("新版本二进制已就位，等待重启")

	if progress != nil {
		progress(100, "更新完成，程序即将重启")
	}
	return nil
}

// installNewBinary 将发布包中的新二进制原子替换当前运行的可执行文件
func installNewBinary(newBinary string) error {
	if _, err := os.Stat(newBinary); err != nil {
		return fmt.Errorf("发布包中未找到新版本二进制 %s: %w", newBinary, err)
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位当前可执行文件: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("无法解析当前可执行文件路径: %w", err)
	}
	tmp := exePath + ".new"
	if err := os.Rename(newBinary, tmp); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func downloadFile(url string, dest string) error {
	resp, err := updateHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("关闭resp.Body失败:", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，HTTP 状态码: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			logx.Error("关闭out失败:", err)
		}
	}(out)

	written, err := io.Copy(out, io.LimitReader(resp.Body, maxExtractBytes+1))
	if err != nil {
		return err
	}
	if written > maxExtractBytes {
		return fmt.Errorf("下载文件超过大小限制")
	}
	return nil
}

// verifyChecksum 从发布包自带的 checksums-<arch>.txt 中校验下载文件的 SHA256。
// 校验和文件缺失或网络错误时拒绝安装（fail-closed），保证更新完整性。
func verifyChecksum(dest string, checksumURL string) error {
	resp, err := updateHTTPClient.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("获取校验和失败: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logx.Error("关闭resp.Body失败:", err)
		}
	}(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("发布包未附带校验和文件，为安全起见拒绝更新")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取校验和失败: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取校验和失败: %w", err)
	}

	targetName := fmt.Sprintf("dockerCopilot-%s.tar.gz", runtime.GOARCH)
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == targetName {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("校验和文件中未找到 %s 的校验值，拒绝解压", targetName)
	}
	expectedSum, err := hex.DecodeString(expected)
	if err != nil || len(expectedSum) != sha256.Size {
		return fmt.Errorf("校验和文件格式非法")
	}

	f, err := os.Open(dest)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logx.Error("关闭file失败:", err)
		}
	}(f)
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(h.Sum(nil), expectedSum) != 1 {
		return fmt.Errorf("下载文件校验失败，可能存在篡改或下载不完整")
	}
	logx.Info("校验和验证通过")
	return nil
}

func decompressTarGz(gzFilePath string, dest string) error {
	file, err := os.Open(gzFilePath)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logx.Error("关闭file失败:", err)
		}
	}(file)

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func(gzr *gzip.Reader) {
		err := gzr.Close()
		if err != nil {
			logx.Error("关闭gzr失败:", err)
		}
	}(gzr)

	tarReader := tar.NewReader(gzr)

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	destAbs = filepath.Clean(destAbs)
	destPrefix := destAbs + string(os.PathSeparator)

	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return fmt.Errorf("压缩包包含不支持的条目类型: %s (%c)", header.Name, header.Typeflag)
		}
		if filepath.IsAbs(header.Name) {
			return fmt.Errorf("压缩包包含绝对路径条目: %s", header.Name)
		}
		target := filepath.Clean(filepath.Join(destAbs, header.Name))
		if target != destAbs && !strings.HasPrefix(target, destPrefix) {
			return fmt.Errorf("压缩包包含越界路径: %s", header.Name)
		}
		if header.Size < 0 || totalBytes+header.Size > maxExtractBytes {
			return fmt.Errorf("压缩包解压内容超过大小限制")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)&os.ModePerm); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)&os.ModePerm)
			if err != nil {
				return err
			}
			written, copyErr := io.Copy(outFile, io.LimitReader(tarReader, header.Size+1))
			closeErr := outFile.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if written > header.Size {
				return fmt.Errorf("压缩包条目数据超过声明大小: %s", header.Name)
			}
			totalBytes += written
		default:
			return fmt.Errorf("未知类型: %v in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
