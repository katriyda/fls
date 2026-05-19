package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"fls/web"
)

var templates map[string]*template.Template

func init() {
	templates = make(map[string]*template.Template)

	entries, err := web.FS.ReadDir("templates")
	if err != nil {
		panic("failed to read templates: " + err.Error())
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".html")
		path := path.Join("templates", entry.Name())

		tmpl := template.Must(
			template.New("").Funcs(template.FuncMap{
				"bytes": formatBytes,
			}).ParseFS(web.FS, path, "templates/layout.html"),
		)
		templates[name] = tmpl
	}
}

// StaticHandler serves embedded static files.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic("failed to create static sub-filesystem: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}

// RenderTemplate renders a template with the given data.
func RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
