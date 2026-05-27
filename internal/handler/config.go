package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fls/internal/config"

	"github.com/alexedwards/scs/v2"
)

type ConfigHandler struct {
	cfg *config.Config
	sm  *scs.SessionManager
}

func NewConfigHandler(cfg *config.Config, sm *scs.SessionManager) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, sm: sm}
}

type configPageData struct {
	Authenticated      bool
	Flash              string
	Port               int
	DataDir            string
	TokenLength        int
	MaxUploadSize      int64
	DefaultExpiry      time.Duration
	SessionTimeout     time.Duration
	LogRetentionDays   int
	RateLimitPerMinute int
	PublicBaseURL      string
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.render(w, "")
}

func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderWithFlash(w, "解析表单失败: "+err.Error())
		return
	}

	// Parse and validate all form values before acquiring the lock
	var newSessionTimeout time.Duration
	var updateSession bool

	type fieldUpdate struct {
		apply func()
	}
	var updates []fieldUpdate

	if v := r.FormValue("max_upload_size"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed >= 1<<20 {
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.MaxUploadSize = p }})
		}
	}
	if v := r.FormValue("token_length"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 4 && parsed <= 64 {
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.TokenLength = p }})
		}
	}
	if v := r.FormValue("default_expiry"); v != "" {
		if v == "never" {
			updates = append(updates, fieldUpdate{func() { h.cfg.DefaultExpiry = 0 }})
		} else if parsed, err := time.ParseDuration(v); err == nil && parsed >= 0 {
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.DefaultExpiry = p }})
		}
	}
	if v := r.FormValue("session_timeout"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil && parsed >= time.Minute {
			newSessionTimeout = parsed
			updateSession = true
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.SessionTimeout = p }})
		}
	}
	if v := r.FormValue("log_retention_days"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.LogRetentionDays = p }})
		}
	}
	if v := r.FormValue("rate_limit_per_minute"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 {
			p := parsed
			updates = append(updates, fieldUpdate{func() { h.cfg.RateLimitPerMinute = p }})
		}
	}
	if v := r.FormValue("public_base_url"); v != "" {
		updates = append(updates, fieldUpdate{func() { h.cfg.PublicBaseURL = v }})
	} else {
		updates = append(updates, fieldUpdate{func() { h.cfg.PublicBaseURL = "" }})
	}

	// Apply all updates and save atomically under a single write lock
	if err := h.cfg.UpdateAndSave(func() {
		for _, u := range updates {
			u.apply()
		}
		if updateSession && h.sm != nil {
			// NOTE: scs.Lifetime is read by the session middleware without synchronization.
			// This is a benign data race — time.Duration is an int64, and aligned 64-bit
			// writes are atomic on all supported platforms. The scs library provides no
			// synchronized setter for this field.
			h.sm.Lifetime = newSessionTimeout
		}
	}); err != nil {
		h.renderWithFlash(w, "保存失败: "+err.Error())
		return
	}

	h.renderWithFlash(w, "配置已保存")
}

func (h *ConfigHandler) renderWithFlash(w http.ResponseWriter, flash string) {
	h.render(w, flash)
}

func (h *ConfigHandler) render(w http.ResponseWriter, flash string) {
	snap := h.cfg.Snapshot()
	RenderTemplate(w, "config", configPageData{
		Authenticated:      true,
		Flash:              flash,
		Port:               snap.Port,
		DataDir:            snap.DataDir,
		TokenLength:        snap.TokenLength,
		MaxUploadSize:      snap.MaxUploadSize,
		DefaultExpiry:      snap.DefaultExpiry,
		SessionTimeout:     snap.SessionTimeout,
		LogRetentionDays:   snap.LogRetentionDays,
		RateLimitPerMinute: snap.RateLimitPerMinute,
		PublicBaseURL:      snap.PublicBaseURL,
	})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return d.String()
}
