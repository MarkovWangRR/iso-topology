//go:build !js

package isotopo

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
)

// inlineLocalIcon (native) turns a local image-file path into a self-contained
// data URI so the icon renders standalone. iso://, data: and http(s) values
// pass through untouched; an unreadable path or unknown extension is left as-is
// (the href simply won't resolve).
func inlineLocalIcon(icon string) string {
	if icon == "" ||
		strings.HasPrefix(icon, "data:") ||
		strings.HasPrefix(icon, "http://") ||
		strings.HasPrefix(icon, "https://") ||
		strings.HasPrefix(icon, "iso://") {
		return icon
	}
	p := strings.TrimPrefix(icon, "file://")
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	var mime string
	switch strings.ToLower(filepath.Ext(p)) {
	case ".svg":
		mime = "image/svg+xml"
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	default:
		return icon
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return icon
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
