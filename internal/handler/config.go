package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fls/internal/config"
)

type ConfigHandler struct {
	cfg *config.Config
}

func NewConfigHandler(cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{cfg: cfg}
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
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.render(w, "")
}

func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderWithFlash(w, "解析表单失败: "+err.Error())
		return
	}

	if v := r.FormValue("max_upload_size"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			h.cfg.MaxUploadSize = parsed
		}
	}
	if v := r.FormValue("token_length"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			h.cfg.TokenLength = parsed
		}
	}
	if v := r.FormValue("default_expiry"); v != "" {
		if v == "never" {
			h.cfg.DefaultExpiry = 0
		} else if parsed, err := time.ParseDuration(v); err == nil {
			h.cfg.DefaultExpiry = parsed
		}
	}
	if v := r.FormValue("session_timeout"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			h.cfg.SessionTimeout = parsed
		}
	}
	if v := r.FormValue("log_retention_days"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			h.cfg.LogRetentionDays = parsed
		}
	}
	if v := r.FormValue("rate_limit_per_minute"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			h.cfg.RateLimitPerMinute = parsed
		}
	}

	if err := h.cfg.Save(); err != nil {
		h.renderWithFlash(w, "保存失败: "+err.Error())
		return
	}

	h.renderWithFlash(w, "配置已保存")
}

func (h *ConfigHandler) renderWithFlash(w http.ResponseWriter, flash string) {
	h.render(w, flash)
}

func (h *ConfigHandler) render(w http.ResponseWriter, flash string) {
	RenderTemplate(w, "config", configPageData{
		Authenticated:      true,
		Flash:              flash,
		Port:               h.cfg.Port,
		DataDir:            h.cfg.DataDir,
		TokenLength:        h.cfg.TokenLength,
		MaxUploadSize:      h.cfg.MaxUploadSize,
		DefaultExpiry:      h.cfg.DefaultExpiry,
		SessionTimeout:     h.cfg.SessionTimeout,
		LogRetentionDays:   h.cfg.LogRetentionDays,
		RateLimitPerMinute: h.cfg.RateLimitPerMinute,
	})
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return d.String()
}
