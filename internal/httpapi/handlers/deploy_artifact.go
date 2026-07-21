package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
const deployArtifactDirEnv = "AEGIS_DEPLOY_ARTIFACT_DIR"
const deployArtifactManifestEnv = "AEGIS_DEPLOY_ARTIFACT_MANIFEST"
const deployArtifactURLEnv = "AEGIS_DEPLOY_ARTIFACT_URL"
const deployArtifactURLTemplateEnv = "AEGIS_DEPLOY_ARTIFACT_URL_TEMPLATE"
const deployArtifactSHA256Env = "AEGIS_DEPLOY_ARTIFACT_SHA256"
const defaultDeployArtifactURLTemplate = "https://raw.githubusercontent.com/PENG1028/aegis/eabcfa665e2bd60629167eb8fb60c37dedb1464a/aegis-{os}-{arch}"
const defaultLinuxAMD64SHA256 = "331744a2c2dd02dd985fa9017e27eeecf01803164fcd13d39a871203df538dbb"

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
	readFile   func(string) ([]byte, error)
	download   func(context.Context, string) (string, func() error, error)
}

type deployArtifactManifest struct {
	Artifacts []deployArtifactManifestEntry `json:"artifacts"`
}

type deployArtifactManifestEntry struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Path   string `json:"path,omitempty"`
	URL    string `json:"url,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

func newLocalAegisArtifactProvider() localAegisArtifactProvider {
	return localAegisArtifactProvider{
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		executable: os.Executable,
		getenv:     os.Getenv,
		stat:       os.Stat,
		readFile:   os.ReadFile,
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
	if p.readFile == nil {
		p.readFile = os.ReadFile
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
	if artifact, err := p.resolveFromManifest(ctx, targetOS, targetArch); artifact != nil || err != nil {
		return artifact, err
	}
	if artifact := p.resolveFromDir(targetOS, targetArch); artifact != nil {
		return artifact, nil
	}
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
		return nil, fmt.Errorf("target platform is %s/%s but current aegis binary is %s/%s; set %s to a matching target binary, %s to an artifact directory, %s to a manifest, or %s to an artifact URL", targetOS, targetArch, hostOS, hostArch, deployArtifactEnv, deployArtifactDirEnv, deployArtifactManifestEnv, deployArtifactURLEnv)
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

func (p localAegisArtifactProvider) resolveFromManifest(ctx context.Context, targetOS, targetArch string) (*aegisArtifact, error) {
	manifestPath := strings.TrimSpace(p.getenv(deployArtifactManifestEnv))
	if manifestPath == "" || targetOS == "" || targetArch == "" {
		return nil, nil
	}
	data, err := p.readFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read %s=%s: %w", deployArtifactManifestEnv, manifestPath, err)
	}
	var manifest deployArtifactManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s=%s: %w", deployArtifactManifestEnv, manifestPath, err)
	}
	for _, entry := range manifest.Artifacts {
		if normalizeOS(entry.OS) != targetOS || normalizeArch(entry.Arch) != targetArch {
			continue
		}
		if local := strings.TrimSpace(entry.Path); local != "" {
			if _, err := p.stat(local); err != nil {
				return nil, fmt.Errorf("manifest artifact %s/%s path %s is not readable: %w", targetOS, targetArch, local, err)
			}
			return &aegisArtifact{LocalPath: local, Source: deployArtifactManifestEnv + ":" + local}, nil
		}
		if url := strings.TrimSpace(entry.URL); url != "" {
			path, cleanup, err := p.download(ctx, url)
			if err != nil {
				return nil, err
			}
			if err := verifyArtifactSHA256(path, entry.SHA256); err != nil {
				cleanup()
				return nil, err
			}
			return &aegisArtifact{LocalPath: path, Source: deployArtifactManifestEnv + ":" + url, Cleanup: cleanup}, nil
		}
		return nil, fmt.Errorf("manifest artifact %s/%s has neither path nor url", targetOS, targetArch)
	}
	return nil, fmt.Errorf("manifest %s has no artifact for %s/%s", manifestPath, targetOS, targetArch)
}

func (p localAegisArtifactProvider) resolveFromDir(targetOS, targetArch string) *aegisArtifact {
	dir := strings.TrimSpace(p.getenv(deployArtifactDirEnv))
	if dir == "" || targetOS == "" || targetArch == "" {
		return nil
	}
	for _, name := range artifactNames(targetOS, targetArch) {
		path := filepath.Join(dir, name)
		if _, err := p.stat(path); err == nil {
			return &aegisArtifact{LocalPath: path, Source: deployArtifactDirEnv + ":" + path}
		}
	}
	return nil
}

func artifactNames(targetOS, targetArch string) []string {
	base := "aegis-" + targetOS + "-" + targetArch
	if targetOS == "windows" {
		return []string{base + ".exe", base}
	}
	return []string{base}
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
