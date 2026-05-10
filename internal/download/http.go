package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ProgressFn receives byte counts during download.
type ProgressFn func(downloaded, total int64)

// httpDownload streams url → dst with atomic .part rename.
func httpDownload(ctx context.Context, c *http.Client, url, dst string, onProgress ProgressFn) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	// No fsync: a 50 MB video pays ~50-200 ms per file on HDD, 5-30 ms on SSD.
	// Atomic rename only promotes fully written content to the final name; on
	// soft errors the deferred Remove cleans the .part. Durability across
	// power loss is not a goal for a desktop video downloader — a crashed
	// .part is just disk waste, never appears as a "complete" file.
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	return os.Rename(tmp, dst)
}

// defaultClient with generous timeouts for large videos.
func defaultClient() *http.Client {
	return &http.Client{
		Timeout: 0, // no overall timeout — large downloads
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		},
	}
}
