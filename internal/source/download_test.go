package source

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("content"))
	if err := VerifyFile(path, fmt.Sprintf("sha256:%x", sum)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(path, "sha256:deadbeef"); err == nil {
		t.Fatal("expected mismatch")
	}
	if err := VerifyFile(path, "md5:deadbeef"); err == nil {
		t.Fatal("expected unsupported algorithm")
	}
}
