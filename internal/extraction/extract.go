package extraction

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type Limits struct {
	MaxExtractedBytes int64
	MaxFiles          int
	MaxFileBytes      int64
	MaxPathBytes      int
	MaxDepth          int
}

func ExtractTarGzip(archivePath, destination string, limits Limits) error {
	// #nosec G304 -- archivePath is a controller-created file verified immediately before extraction.
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return extract(tar.NewReader(gz), destination, limits)
}

func extract(tr *tar.Reader, destination string, limits Limits) error {
	if limits.MaxFiles <= 0 || limits.MaxExtractedBytes <= 0 || limits.MaxFileBytes <= 0 {
		return errors.New("extraction limits must be positive")
	}
	seen, folded := map[string]struct{}{}, map[string]string{}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return fmt.Errorf("open extraction root: %w", err)
	}
	defer func() { _ = root.Close() }()
	var total int64
	for count := 0; ; count++ {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if count >= limits.MaxFiles {
			return fmt.Errorf("archive exceeds %d files", limits.MaxFiles)
		}
		name, err := safeName(h.Name, limits)
		if err != nil {
			return err
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate archive path %q", name)
		}
		seen[name] = struct{}{}
		fold := strings.ToLower(name)
		if previous, ok := folded[fold]; ok && previous != name {
			return fmt.Errorf("case-folding collision between %q and %q", previous, name)
		}
		folded[fold] = name
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := beneath(destination, target); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, 0:
			if h.Size < 0 || h.Size > limits.MaxFileBytes {
				return fmt.Errorf("archive file %q exceeds size limit", name)
			}
			if total > limits.MaxExtractedBytes-h.Size {
				return fmt.Errorf("archive exceeds extracted size limit")
			}
			total += h.Size
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return err
			}
			out, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(out, tr, h.Size)
			closeErr := out.Close()
			if copyErr != nil {
				return fmt.Errorf("extract %q: %w", name, copyErr)
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported archive entry %q (type %d)", name, h.Typeflag)
		}
	}
}

func safeName(name string, limits Limits) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	if limits.MaxPathBytes > 0 && len(clean) > limits.MaxPathBytes {
		return "", fmt.Errorf("archive path is too long")
	}
	if limits.MaxDepth > 0 && strings.Count(clean, "/")+1 > limits.MaxDepth {
		return "", fmt.Errorf("archive path is too deep")
	}
	return clean, nil
}

func beneath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes extraction root")
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(rel) != "" {
		return fmt.Errorf("path has a volume")
	}
	return nil
}
