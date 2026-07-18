package handlers

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"aegis/internal/deploy"
)

func TestArtifactProviderUsesCurrentBinaryForMatchingPlatform(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:       "linux",
		goarch:     "amd64",
		executable: func() (string, error) { return "/usr/local/bin/aegis", nil },
		getenv:     func(string) string { return "" },
		stat:       func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
	}

	artifact, err := p.Resolve(&deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
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

func TestArtifactProviderRejectsCrossPlatformCurrentBinary(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(string) string { return "" },
		stat:   func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
	}

	_, err := p.Resolve(&deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err == nil {
		t.Fatal("Resolve succeeded, want platform mismatch error")
	}
	if !strings.Contains(err.Error(), deployArtifactEnv) {
		t.Fatalf("error = %q, want %s guidance", err.Error(), deployArtifactEnv)
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

	artifact, err := p.Resolve(&deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
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
