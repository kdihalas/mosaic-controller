package source

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Downloader struct {
	Client   *http.Client
	MaxBytes int64
}

func NewDownloader(timeout time.Duration, maxBytes int64) *Downloader {
	t := &http.Transport{DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: timeout, MaxIdleConns: 20, IdleConnTimeout: 90 * time.Second}
	c := &http.Client{Transport: t, Timeout: timeout}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("unsupported redirect scheme")
		}
		if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return fmt.Errorf("refusing HTTPS downgrade redirect")
		}
		return nil
	}
	return &Downloader{Client: c, MaxBytes: maxBytes}
}

func (d *Downloader) Download(ctx context.Context, rawURL, expectedDigest, destination string) (int64, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return 0, fmt.Errorf("unsupported artifact URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("artifact server returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > d.MaxBytes {
		return 0, fmt.Errorf("artifact exceeds compressed size limit")
	}
	// #nosec G304 -- destination is a controller-created path inside its private temporary directory.
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	success := false
	defer func() {
		if !success {
			_ = os.Remove(destination)
		}
	}()
	written, copyErr := io.Copy(out, io.LimitReader(resp.Body, d.MaxBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return written, copyErr
	}
	if closeErr != nil {
		return written, closeErr
	}
	if written > d.MaxBytes {
		return written, fmt.Errorf("artifact exceeds compressed size limit")
	}
	if err := VerifyFile(destination, expectedDigest); err != nil {
		return written, err
	}
	success = true
	return written, nil
}

func VerifyFile(file, expected string) error {
	algorithm, value, ok := strings.Cut(expected, ":")
	if !ok || value == "" {
		return fmt.Errorf("invalid artifact digest")
	}
	// #nosec G304 -- callers pass a controller-created temporary or Artifact SDK storage path.
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var h io.Writer
	var sum func() string
	switch algorithm {
	case "sha256":
		x := sha256.New()
		h = x
		sum = func() string { return hex.EncodeToString(x.Sum(nil)) }
	case "sha512":
		x := sha512.New()
		h = x
		sum = func() string { return hex.EncodeToString(x.Sum(nil)) }
	default:
		return fmt.Errorf("unsupported digest algorithm %q", algorithm)
	}
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if actual := sum(); actual != strings.ToLower(value) {
		return fmt.Errorf("artifact digest mismatch: expected %s:%s, got %s:%s", algorithm, value, algorithm, actual)
	}
	return nil
}
