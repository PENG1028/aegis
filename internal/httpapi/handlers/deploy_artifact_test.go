package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
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
			want := "https://raw.githubusercontent.com/PENG1028/aegis/4808c3990b5712750f1038def58e2a76d5228207/aegis-linux-amd64"
			if url != want {
				t.Fatalf("url = %q, want %q", url, want)
			}
			path := writeTempArtifact(t, []byte("binary"))
			return path, func() error { return os.Remove(path) }, nil
		},
	}
	p.getenv = func(key string) string {
		if key == deployArtifactSHA256Env {
			return sha256Hex([]byte("binary"))
		}
		return ""
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(filepath.Base(artifact.LocalPath), "aegis-test-artifact") {
		t.Fatalf("LocalPath = %q, want downloaded artifact", artifact.LocalPath)
	}
	if artifact.Source != "https://raw.githubusercontent.com/PENG1028/aegis/4808c3990b5712750f1038def58e2a76d5228207/aegis-linux-amd64" {
		t.Fatalf("Source = %q, want default GitHub URL", artifact.Source)
	}
	if artifact.Cleanup != nil {
		artifact.Cleanup()
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

func TestArtifactProviderRejectsChecksumMismatch(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactSHA256Env {
				return sha256Hex([]byte("expected"))
			}
			return ""
		},
		stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		download: func(ctx context.Context, url string) (string, func() error, error) {
			path := writeTempArtifact(t, []byte("actual"))
			return path, func() error { return os.Remove(path) }, nil
		},
	}

	_, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err == nil {
		t.Fatal("Resolve succeeded, want checksum mismatch")
	}
	if !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Fatalf("error = %q, want SHA256 mismatch", err.Error())
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
	if !strings.Contains(err.Error(), deployArtifactDirEnv) {
		t.Fatalf("error = %q, want %s guidance", err.Error(), deployArtifactDirEnv)
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

func TestArtifactProviderUsesArtifactDirectory(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactDirEnv {
				return "F:/aegis-artifacts"
			}
			return ""
		},
		stat: func(path string) (os.FileInfo, error) {
			if filepath.ToSlash(path) != "F:/aegis-artifacts/aegis-linux-amd64" {
				return nil, os.ErrNotExist
			}
			return fakeFileInfo{}, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if filepath.ToSlash(artifact.LocalPath) != "F:/aegis-artifacts/aegis-linux-amd64" {
		t.Fatalf("LocalPath = %q, want artifact directory match", artifact.LocalPath)
	}
	if !strings.HasPrefix(artifact.Source, deployArtifactDirEnv+":") {
		t.Fatalf("Source = %q, want %s source", artifact.Source, deployArtifactDirEnv)
	}
}

func TestArtifactProviderUsesManifestLocalArtifact(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactManifestEnv {
				return "F:/aegis-artifacts/manifest.json"
			}
			return ""
		},
		readFile: func(path string) ([]byte, error) {
			if path != "F:/aegis-artifacts/manifest.json" {
				return nil, os.ErrNotExist
			}
			return []byte(`{"artifacts":[{"os":"linux","arch":"x86_64","path":"F:/aegis-artifacts/aegis-linux-amd64"}]}`), nil
		},
		stat: func(path string) (os.FileInfo, error) {
			if path != "F:/aegis-artifacts/aegis-linux-amd64" {
				return nil, os.ErrNotExist
			}
			return fakeFileInfo{}, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.LocalPath != "F:/aegis-artifacts/aegis-linux-amd64" {
		t.Fatalf("LocalPath = %q, want manifest path", artifact.LocalPath)
	}
	if artifact.Source != deployArtifactManifestEnv+":F:/aegis-artifacts/aegis-linux-amd64" {
		t.Fatalf("Source = %q, want manifest source", artifact.Source)
	}
}

func TestArtifactProviderUsesManifestURLWithChecksum(t *testing.T) {
	p := localAegisArtifactProvider{
		goos:   "windows",
		goarch: "amd64",
		getenv: func(key string) string {
			if key == deployArtifactManifestEnv {
				return "F:/aegis-artifacts/manifest.json"
			}
			return ""
		},
		readFile: func(path string) ([]byte, error) {
			return []byte(`{"artifacts":[{"os":"linux","arch":"amd64","url":"https://example.invalid/aegis-linux-amd64","sha256":"` + sha256Hex([]byte("binary")) + `"}]}`), nil
		},
		stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		download: func(ctx context.Context, url string) (string, func() error, error) {
			if url != "https://example.invalid/aegis-linux-amd64" {
				t.Fatalf("url = %q, want manifest URL", url)
			}
			path := writeTempArtifact(t, []byte("binary"))
			return path, func() error { return os.Remove(path) }, nil
		},
	}

	artifact, err := p.Resolve(context.Background(), &deploy.PreflightReport{Host: &deploy.HostInfo{OS: "linux", Arch: "x86_64"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if artifact.Source != deployArtifactManifestEnv+":https://example.invalid/aegis-linux-amd64" {
		t.Fatalf("Source = %q, want manifest URL source", artifact.Source)
	}
	if artifact.Cleanup != nil {
		artifact.Cleanup()
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

func writeTempArtifact(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "aegis-test-artifact-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		t.Fatal(err)
	}
	return f.Name()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
