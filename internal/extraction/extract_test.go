package extraction

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRejectsUnsafeEntries(t *testing.T) {
	limits := Limits{MaxExtractedBytes: 1024, MaxFiles: 10, MaxFileBytes: 1024, MaxPathBytes: 100, MaxDepth: 10}
	for _, tc := range []struct {
		name   string
		header tar.Header
	}{
		{"traversal", tar.Header{Name: "../escape", Typeflag: tar.TypeReg}},
		{"symlink", tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target"}},
		{"windows", tar.Header{Name: `C:\\escape`, Typeflag: tar.TypeReg}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw bytes.Buffer
			tw := tar.NewWriter(&raw)
			if err := tw.WriteHeader(&tc.header); err != nil {
				t.Fatal(err)
			}
			_ = tw.Close()
			if err := extract(tar.NewReader(bytes.NewReader(raw.Bytes())), t.TempDir(), limits); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestExtractRegularFile(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	data := []byte("hello")
	if err := tw.WriteHeader(&tar.Header{Name: "dir/file", Typeflag: tar.TypeReg, Size: int64(len(data)), Mode: 0o777}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write(data)
	_ = tw.Close()
	dir := t.TempDir()
	if err := extract(tar.NewReader(bytes.NewReader(raw.Bytes())), dir, Limits{MaxExtractedBytes: 10, MaxFiles: 2, MaxFileBytes: 10}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "dir", "file"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}
