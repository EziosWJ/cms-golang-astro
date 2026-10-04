// Package staticweb serves immutable whitelisted artifacts without database access.
package staticweb

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Handler struct{ Root string }

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, e := deployment.Read(h.Root)
	if e != nil || p == nil {
		http.Error(w, "站点尚未发布", 503)
		return
	}
	Serve(w, r, filepath.Join(h.Root, "releases", p.ReleaseKey), builder.Marker{AttemptID: p.AttemptID, ReleaseKey: p.ReleaseKey, ManifestHash: p.ManifestHash}, r.URL.Path)
}
func Serve(w http.ResponseWriter, r *http.Request, root string, expected builder.Marker, requestPath string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", 405)
		return
	}
	raw, e := os.ReadFile(filepath.Join(root, ".release.json"))
	var marker builder.Marker
	if e != nil || json.Unmarshal(raw, &marker) != nil || marker.AttemptID != expected.AttemptID || marker.ReleaseKey != expected.ReleaseKey || marker.ManifestHash != expected.ManifestHash {
		http.Error(w, "站点产物无法核实", 503)
		return
	}
	relative := strings.TrimPrefix(requestPath, "/")
	for _, segment := range strings.Split(relative, "/") {
		if strings.HasPrefix(segment, ".") || strings.ContainsAny(segment, "\\\x00") {
			http.NotFound(w, r)
			return
		}
	}
	if path.Clean("/"+relative) != "/"+strings.TrimSuffix(relative, "/") && relative != "" {
		http.NotFound(w, r)
		return
	}
	if relative == "" || strings.HasSuffix(relative, "/") {
		relative += "index.html"
	} else if _, ok := marker.Files[relative+"/index.html"]; ok {
		http.Redirect(w, r, r.URL.Path+"/", 308)
		return
	}
	hash, ok := marker.Files[relative]
	status := 200
	if !ok {
		relative = "404.html"
		hash, ok = marker.Files[relative]
		status = 404
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	bytes, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	sum := sha256.Sum256(bytes)
	if e != nil || hex.EncodeToString(sum[:]) != hash {
		http.Error(w, "产物校验失败", 503)
		return
	}
	if filename, download := marker.Downloads[relative]; download {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}
	if status == 404 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if r.Method == "GET" {
			_, _ = w.Write(bytes)
		}
		return
	}
	http.ServeContent(w, r, relative, zeroTime, strings.NewReader(string(bytes)))
}

var zeroTime = func() (t time.Time) { return }()
