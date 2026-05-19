package handler

import (
	"net/http"

	"fls/internal/middleware"
	"fls/internal/service"

	"github.com/alexedwards/scs/v2"
)

type LoginHandler struct {
	Auth            *service.Auth
	SessionManager  *scs.SessionManager
	DataDir         string
}

func (h *LoginHandler) GetLogin(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"CSRFToken": middleware.CSRFToken(r),
	}
	RenderTemplate(w, "login", data)
}

func (h *LoginHandler) PostLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		data := map[string]interface{}{
			"CSRFToken": middleware.CSRFToken(r),
			"Error":     "invalid form data",
		}
		w.WriteHeader(http.StatusBadRequest)
		RenderTemplate(w, "login", data)
		return
	}

	password := r.FormValue("password")
	valid, err := h.Auth.VerifyPassword(password)
	if err != nil {
		data := map[string]interface{}{
			"CSRFToken": middleware.CSRFToken(r),
			"Error":     "internal error",
		}
		w.WriteHeader(http.StatusInternalServerError)
		RenderTemplate(w, "login", data)
		return
	}

	if !valid {
		data := map[string]interface{}{
			"CSRFToken": middleware.CSRFToken(r),
			"Error":     "密码错误",
		}
		w.WriteHeader(http.StatusUnauthorized)
		RenderTemplate(w, "login", data)
		return
	}

	middleware.SetAuthenticated(r.Context(), h.SessionManager)
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}
