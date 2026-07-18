package handlers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"aegis/internal/deploy"
)

func TestArtifactProviderUsesDefaultGitHubArtifactForLinuxAMD64(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(string) string { return "" },
		stat:   func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		download: func(ctx context.Context, url string) (string, func() error, error) {
			want := "https://raw.githubusercontent.com/PENG1028/aegis/codex/aegis-provider-safe-bin/aegis-linux-amd64"
			if url != want {
				t.Fatalf("url = %q, want %q", url, want)
			}
			return "C:/Temp/aegis-linux-amd64", nil, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.LocalPath != "C:/Temp/aegis-linux-amd64" {
		t.Fatalf("LocalPath = %q, want downloaded artifact", artifact.LocalPath)
	}
	if artifact.Source != "https://raw.githubusercontent.com/PENG1028/aegis/codex/aegis-provider-safe-bin/aegis-linux-amd64" {
		t.Fatalf("Source = %q, want default GitHub URL", artifact.Source)
	}
}

func TestArtifactProviderUsesCurrentBinaryWhenTargetPlatformUnknown(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:       "linux",
		goarch:     "amd64",
		executable: func() (string, error) { return "/usr/local/bin/aegis", nil },
		getenv:     func(string) string { return "" },
		stat:       func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.LocalPath != "/usr/local/bin/aegis" {
		t.Fatalf("LocalPath = %q, want current binary", artifact.LocalPath)
	}
	if artifact.Source != "current_binary" {
		t.Fatalf("Source = %q, want current_binary", artifact.Source)
	}
}

func TestArtifactProviderRejectsUnsupportedCrossPlatformCurrentBinary(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(string) string { return "" },
		stat:   func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
	}

	_, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "aarch64"}})
	if err == nil {
		t.Fatal("Resolve succeeded, want platform mismatch error")
	}
	if !strings.Contains(err.Error(), deployArtifactURLEnv) {
		t.Fatalf("error = %q, want %s guidance", err.Error(), deployArtifactURLEnv)
	}
}

func TestArtifactProviderUsesExplicitArtifactForCrossPlatform(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactEnv {
				return "F:/bin/aegis-linux-amd64"
			}
			return ""
		},
		stat: func(path string) (os.FileInfo, error) {
			if path != "F:/bin/aegis-linux-amd64" {
				return nil, errors.New("unexpected path")
			}
			return fakeFileInfo{}, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.LocalPath != "F:/bin/aegis-linux-amd64" {
		t.Fatalf("LocalPath = %q, want explicit artifact", artifact.LocalPath)
	}
	if artifact.Source != deployArtifactEnv {
		t.Fatalf("Source = %q, want %s", artifact.Source, deployArtifactEnv)
	}
}

func TestArtifactProviderUsesExplicitArtifactURL(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactURLEnv {
				return "https://example.invalid/aegis-linux-amd64"
			}
			return ""
		},
		stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		download: func(ctx context.Context, url string) (string, func() error, error) {
			if url != "https://example.invalid/aegis-linux-amd64" {
				t.Fatalf("url = %q, want explicit URL", url)
			}
			return "C:/Temp/aegis", nil, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.Source != "https://example.invalid/aegis-linux-amd64" {
		t.Fatalf("Source = %q, want explicit URL", artifact.Source)
	}
}

func TestArtifactProviderUsesURLTemplate(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactURLTemplateEnv {
				return "https://example.invalid/aegis-{os}-{arch}"
			}
			return ""
		},
		stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		download: func(ctx context.Context, url string) (string, func() error, error) {
			if url != "https://example.invalid/aegis-linux-amd64" {
				t.Fatalf("url = %q, want expanded template", url)
			}
			return "C:/Temp/aegis", nil, nil
		},
	}

	_, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}

func TestNormalizePlatform(t *testing.T) {
	osName, arch := targetPlatform(&deploy.PreflightReport{Host: &deploy.HostInfo{OS: "Linux", Arch: "aarch64"}})
	if osName != "linux" || arch != "arm64" {
		t.Fatalf("platform = %s/%s, want linux/arm64", osName, arch)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "aegis" }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
