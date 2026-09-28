package httpapi

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

func serveStatic(web fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}

		data, err := fs.ReadFile(web, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			ctype = "application/octet-stream"
		}
		if name == "index.html" {
			ctype = "text/html; charset=utf-8"
		}
		w.Header().Set("Content-Type", ctype)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", itoa(len(data)))
			return
		}
		_, _ = w.Write(data)
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
