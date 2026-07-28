package certstore

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestConfigureConsumerGIDRepairsAndPreservesAssetAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix ownership semantics")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.Atoi(current.Gid)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "certs")
	if err := os.MkdirAll(dir, privateCertDirMode); err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "existing.crt")
	keyPath := filepath.Join(dir, "existing.key")
	unrelatedPath := filepath.Join(dir, "note.txt")
	for _, path := range []string{certPath, keyPath, unrelatedPath} {
		if err := os.WriteFile(path, []byte("old"), privateCertFileMode); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(nil, dir)
	if err := svc.configureConsumerGID(gid); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, sharedCertDirMode)
	assertMode(t, certPath, sharedCertFileMode)
	assertMode(t, keyPath, sharedCertFileMode)
	assertMode(t, unrelatedPath, privateCertFileMode)
	newPath := filepath.Join(dir, "uploaded.key")
	if err := svc.writeAssetFile(newPath, []byte("uploaded")); err != nil {
		t.Fatal(err)
	}
	assertMode(t, newPath, sharedCertFileMode)

	if err := svc.replaceFile(keyPath, []byte("renewed")); err != nil {
		t.Fatal(err)
	}
	assertMode(t, keyPath, sharedCertFileMode)
	content, err := os.ReadFile(keyPath)
	if err != nil || string(content) != "renewed" {
		t.Fatalf("replacement content = %q, err = %v", content, err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}
