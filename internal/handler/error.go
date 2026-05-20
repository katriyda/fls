package handler

import (
	"log"
	"net/http"
)

type ErrorData struct {
	Title         string
	Code          int
	Message       string
	Authenticated bool
}

func renderError(w http.ResponseWriter, code int, title, message string) {
	tmpl, ok := templates["error"]
	if !ok {
		http.Error(w, "template not found: error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	tmpl.ExecuteTemplate(w, "layout.html", ErrorData{
		Title:   title,
		Code:    code,
		Message: message,
	})
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	renderError(w, 404, "页面未找到", "请求的页面不存在")
}

func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	renderError(w, 405, "方法不允许", "请求方法不被允许")
}

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				renderError(w, 500, "服务器错误", "发生内部错误，请稍后重试")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
