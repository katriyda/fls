package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fls/internal/config"

	"github.com/justinas/nosurf"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

type contextKey string

const csrfTokenKey = contextKey("csrf_token")

func CSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(csrfTokenKey).(string); ok {
		return token
	}
	return ""
}

func AddCSRFToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := nosurf.Token(r)
		ctx := context.WithValue(r.Context(), csrfTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CSRFMiddleware(next http.Handler) http.Handler {
	return nosurf.New(AddCSRFToken(next))
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://unpkg.com 'unsafe-inline' 'unsafe-hashes'; style-src 'self' https://unpkg.com https://fonts.googleapis.com; img-src 'self' data:; font-src 'self' https://fonts.gstatic.com; form-action 'self'")
		w.Header().Set("X-XSS-Protection", "0")
		next.ServeHTTP(w, r)
	})
}

var LoginRate = limiter.Rate{
	Limit:  200,
	Period: time.Minute,
}

var APIRate = limiter.Rate{
	Limit:  600,
	Period: time.Minute,
}

// DynamicRateLimitMiddleware creates a thread-safe rate limiter middleware that adjusts its limit dynamically.
func DynamicRateLimitMiddleware(cfg *config.Config, isLogin bool) func(http.Handler) http.Handler {
	store := memory.NewStore()
	var currentLimit int
	var instance *limiter.Limiter
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			limit := 60
			if cfg != nil {
				if isLogin {
					limit = cfg.RateLimitPerMinute / 3
					if limit < 5 {
						limit = 5
					}
				} else {
					limit = cfg.RateLimitPerMinute
				}
			} else {
				if isLogin {
					limit = 200
				} else {
					limit = 600
				}
			}

			if limit != currentLimit || instance == nil {
				currentLimit = limit
				rate := limiter.Rate{
					Limit:  int64(limit),
					Period: time.Minute,
				}
				instance = limiter.New(store, rate)
			}
			lim := instance
			mu.Unlock()

			key := r.RemoteAddr
			lctx, err := lim.Get(r.Context(), key)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", currentLimit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", lctx.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", lctx.Reset))

			if lctx.Reached {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RateLimitMiddleware(rate limiter.Rate) func(http.Handler) http.Handler {
	store := memory.NewStore()
	instance := limiter.New(store, rate)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr

			lctx, err := instance.Get(r.Context(), key)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rate.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", lctx.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", lctx.Reset))

			if lctx.Reached {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ValidatePath(baseDir, targetPath string) error {
	cleanBase := filepath.Clean(baseDir)
	cleanTarget := filepath.Clean(targetPath)

	if !strings.HasSuffix(cleanBase, string(filepath.Separator)) {
		cleanBase += string(filepath.Separator)
	}

	if !strings.HasPrefix(cleanTarget, cleanBase) {
		return fmt.Errorf("path traversal detected: %s is outside %s", targetPath, baseDir)
	}

	return nil
}
