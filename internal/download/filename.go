package download

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// BuildPath builds an absolute output path under outputDir.
// Format: <yyyymmdd-hhmmss>_<taskID-prefix>_<seed>.<ext>.
func BuildPath(outputDir, taskID string, seed int64, ext string) string {
	if ext == "" {
		ext = "mp4"
	}
	ext = strings.TrimPrefix(ext, ".")
	prefix := taskID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	stamp := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("%s_%s_%d.%s", stamp, sanitize(prefix), seed, ext)
	return filepath.Join(outputDir, name)
}

func sanitize(s string) string {
	const bad = `\/:*?"<>|`
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if strings.ContainsRune(bad, r) {
			out = append(out, '_')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
