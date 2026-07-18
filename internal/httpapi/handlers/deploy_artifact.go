package handlers

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"aegis/internal/deploy"
)

const deployArtifactEnv = "AEGIS_DEPLOY_ARTIFACT"

type aegisArtifact struct {
	LocalPath string
	Source    string
}

type localAegisArtifactProvider struct {
	goos       string
	goarch     string
	executable func() (string, error)
	getenv     func(string) string
	stat       func(string) (os.FileInfo, error)
}

func newLocalAegisArtifactProvider() localAegisArtifactProvider {
	return localAegisArtifactProvider{
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		executable: os.Executable,
		getenv:     os.Getenv,
		stat:       os.Stat,
	}
}

func (p localAegisArtifactProvider) Resolve(report *deploy.PreflightReport) (*aegisArtifact, error) {
	if p.getenv == nil {
		p.getenv = os.Getenv
	}
	if p.stat == nil {
		p.stat = os.Stat
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
	if targetOS != "" && targetArch != "" && (targetOS != hostOS || targetArch != hostArch) {
		return nil, fmt.Errorf("target platform is %s/%s but current aegis binary is %s/%s; set %s to a matching target binary", targetOS, targetArch, hostOS, hostArch, deployArtifactEnv)
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
