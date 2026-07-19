package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aegis/internal/deploy"
)

const deployArtifactEnv = "AEGIS_DEPLOY_ARTIFACT"
const deployArtifactURLEnv = "AEGIS_DEPLOY_ARTIFACT_URL"
const deployArtifactURLTemplateEnv = "AEGIS_DEPLOY_ARTIFACT_URL_TEMPLATE"
const deployArtifactSHA256Env = "AEGIS_DEPLOY_ARTIFACT_SHA256"
const defaultDeployArtifactURLTemplate = "https://raw.githubusercontent.com/PENG1028/aegis/4808c3990b5712750f1038def58e2a76d5228207/aegis-{os}-{arch}"
const defaultLinuxAMD64SHA256 = "3c1d26030ce74f3e2f668f70b5e6595155e8eb9a6d555b6dfb50ffd455a6ebd8"

type aegisArtifact struct {
	LocalPath string
	Source    string
	Cleanup   func() error
}

type localAegisArtifactProvider struct {
	goos       string
	goarch     string
	executable func() (string, error)
	getenv     func(string) string
	stat       func(string) (os.FileInfo, error)
	download   func(context.Context, string) (string, func() error, error)
}

func newLocalAegisArtifactProvider() localAegisArtifactProvider {
	return localAegisArtifactProvider{
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		executable: os.Executable,
		getenv:     os.Getenv,
		stat:       os.Stat,
		download:   downloadArtifact,
	}
}

func (p localAegisArtifactProvider) Resolve(ctx context.Context, report *deploy.PreflightReport) (*aegisArtifact, error) {
	if p.getenv == nil {
		p.getenv = os.Getenv
	}
	if p.stat == nil {
		p.stat = os.Stat
	}
	if p.download == nil {
		p.download = downloadArtifact
	}
	if explicit := strings.TrimSpace(p.getenv(deployArtifactEnv)); explicit != "" {
		if _, err := p.stat(explicit); err != nil {
			return nil, fmt.Errorf("%s=%s is not readable: %w", deployArtifactEnv, explicit, err)
		}
		return &aegisArtifact{LocalPath: explicit, Source: deployArtifactEnv}, nil
	}

	targetOS, targetArch := targetPlatform(report)
	hostOS := normalizeOS(p.goos)
	hostArch := normalizeArch(p.goarch)
	if url := p.artifactURL(targetOS, targetArch); url != "" {
		path, cleanup, err := p.download(ctx, url)
		if err != nil {
			return nil, err
		}
		if err := verifyArtifactSHA256(path, p.expectedSHA256(targetOS, targetArch)); err != nil {
			cleanup()
			return nil, err
		}
		return &aegisArtifact{LocalPath: path, Source: url, Cleanup: cleanup}, nil
	}

	if targetOS != "" && targetArch != "" && (targetOS != hostOS || targetArch != hostArch) {
		return nil, fmt.Errorf("target platform is %s/%s but current aegis binary is %s/%s; set %s to a matching target binary or %s to an artifact URL", targetOS, targetArch, hostOS, hostArch, deployArtifactEnv, deployArtifactURLEnv)
	}

	if p.executable == nil {
		p.executable = os.Executable
	}
	path, err := p.executable()
	if err != nil {
		return nil, fmt.Errorf("resolve current aegis binary: %w", err)
	}
	if _, err := p.stat(path); err != nil {
		return nil, fmt.Errorf("current aegis binary is not readable: %w", err)
	}
	return &aegisArtifact{LocalPath: path, Source: "current_binary"}, nil
}

func (p localAegisArtifactProvider) artifactURL(targetOS, targetArch string) string {
	if explicit := strings.TrimSpace(p.getenv(deployArtifactURLEnv)); explicit != "" {
		return explicit
	}
	tmpl := strings.TrimSpace(p.getenv(deployArtifactURLTemplateEnv))
	if tmpl == "" {
		tmpl = defaultDeployArtifactURLTemplate
	}
	if targetOS == "" || targetArch == "" {
		return ""
	}
	if targetOS != "linux" || targetArch != "amd64" {
		return ""
	}
	return strings.NewReplacer("{os}", targetOS, "{arch}", targetArch).Replace(tmpl)
}

func (p localAegisArtifactProvider) expectedSHA256(targetOS, targetArch string) string {
	if explicit := strings.TrimSpace(p.getenv(deployArtifactSHA256Env)); explicit != "" {
		return strings.ToLower(explicit)
	}
	if targetOS == "linux" && targetArch == "amd64" && strings.TrimSpace(p.getenv(deployArtifactURLEnv)) == "" && strings.TrimSpace(p.getenv(deployArtifactURLTemplateEnv)) == "" {
		return defaultLinuxAMD64SHA256
	}
	return ""
}

func verifyArtifactSHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(sum.Sum(nil))
	if actual != expected {
		return fmt.Errorf("artifact SHA256 mismatch: got %s want %s", actual, expected)
	}
	return nil
}

func downloadArtifact(ctx context.Context, url string) (string, func() error, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download artifact %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("download artifact %s: HTTP %d", url, resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "aegis-deploy-*"+filepath.Ext(url))
	if err != nil {
		return "", nil, err
	}
	path := tmp.Name()
	cleanup := func() error { return os.Remove(path) }
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.Chmod(path, 0o755); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func targetPlatform(report *deploy.PreflightReport) (string, string) {
	if report == nil || report.Host == nil {
		return "", ""
	}
	return normalizeOS(report.Host.OS), normalizeArch(report.Host.Arch)
}

func normalizeOS(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "linux", "darwin", "windows":
		return v
	case "mingw", "msys", "cygwin":
		return "windows"
	default:
		return v
	}
}

func normalizeArch(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv6l":
		return "arm"
	default:
		return v
	}
}
