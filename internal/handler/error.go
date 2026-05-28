package handler

import (
	"log"
	"net/http"

	"fls/internal/middleware"

	"github.com/alexedwards/scs/v2"
)

type ErrorData struct {
	Title         string
	Code          int
	Message       string
	Authenticated bool
	CSRFToken     string
}

func renderErrorWithAuth(w http.ResponseWriter, r *http.Request, code int, title, message string, authenticated bool) {
	tmpl, ok := templates["error"]
	if !ok {
		http.Error(w, "template not found: error", http.StatusInternalServerError)
		return
	}
	var csrfToken string
	if r != nil {
		csrfToken = middleware.CSRFToken(r)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	tmpl.ExecuteTemplate(w, "layout.html", ErrorData{
		Title:         title,
		Code:          code,
		Message:       message,
		Authenticated: authenticated,
		CSRFToken:     csrfToken,
	})
}

func NewNotFoundHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated := sm.GetBool(r.Context(), "authenticated")
		renderErrorWithAuth(w, r, 404, "页面未找到", "请求的页面不存在", authenticated)
	}
}

func NewMethodNotAllowedHandler(sm *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated := sm.GetBool(r.Context(), "authenticated")
		renderErrorWithAuth(w, r, 405, "方法不允许", "请求方法不被允许", authenticated)
	}
}

func NewRecoveryMiddleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("panic recovered: %v", err)
					authenticated := false
					func() {
						defer func() { recover() }()
						authenticated = sm.GetBool(r.Context(), "authenticated")
					}()
					renderErrorWithAuth(w, r, 500, "服务器错误", "发生内部错误，请稍后重试", authenticated)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
