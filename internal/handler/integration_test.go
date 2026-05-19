package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fls/internal/database"
	"fls/internal/middleware"
	"fls/internal/service"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// setupTestServer creates a full in-memory test server with middleware and handlers.
func setupTestServer(t *testing.T) (*http.Client, string, func()) {
	t.Helper()

	dir := t.TempDir()
	rdb, err := database.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("database.New() error = %v", err)
	}
	if err := rdb.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}

	// Seed admin password
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.MinCost)
	rdb.DB.Exec(`INSERT OR REPLACE INTO config (key, value, updated_at) VALUES ('admin_password', ?, CURRENT_TIMESTAMP)`, string(hash))

	// Seed test file on disk
	testContent := []byte("integration test file content")
	testPath := filepath.Join(dir, "test-file.txt")
	os.WriteFile(testPath, testContent, 0644)

	fileID := uuid.New().String()
	rdb.DB.Exec(
		`INSERT INTO files (id, filename, original_name, size, mime_type, storage_path, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		fileID, "test-file.txt", "test-file.txt", int64(len(testContent)), "text/plain", testPath,
	)

	shareToken := "testtokn"
	shareID := uuid.New().String()
	rdb.DB.Exec(
		`INSERT INTO shares (id, file_id, token, password_hash, expires_at, max_downloads, download_count, content_type, text_content, created_at, updated_at)
		 VALUES (?, ?, ?, ?, NULL, 0, 0, 'file', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		shareID, fileID, shareToken, "",
	)

	// Services
	authSvc := service.NewAuth(rdb.DB)
	shareSvc := service.NewShareService(rdb.DB)
	statsSvc := service.NewStatsService(rdb.DB)

	// Session
	sm := scs.New()
	sm.Lifetime = 24 * time.Hour

	// Handlers (only the ones registered in routes below)
	loginH := &LoginHandler{Auth: authSvc, SessionManager: sm, DataDir: dir}
	dashH := NewDashboardHandler(statsSvc, shareSvc, rdb.DB)
	dlH := NewDownloadHandler(rdb.DB, shareSvc, statsSvc)
	statsH := NewStatsHandler(statsSvc)

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(RecoveryMiddleware)
	r.Use(chimw.RealIP)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(sm.LoadAndSave)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

	r.Group(func(r chi.Router) {
		r.Handle("/static/*", http.StripPrefix("/static/", StaticHandler()))
	})

	// Login group with CSRF
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimitMiddleware(middleware.LoginRate))
		r.Use(middleware.CSRFMiddleware)
		r.Get("/login", loginH.GetLogin)
		r.Post("/login", loginH.PostLogin)
	})

	// Auth-by-session helper: returns the session cookie token directly
	// so the test can manually add it to requests
	r.Get("/test-login", func(w http.ResponseWriter, r *http.Request) {
		middleware.SetAuthenticated(r.Context(), sm)
		w.Write([]byte("logged in"))
	})

	// Admin group: CSRF + auth
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimitMiddleware(middleware.APIRate))
		r.Use(middleware.CSRFMiddleware)
		r.Use(middleware.AuthMiddleware(sm))
		r.Get("/admin/", dashH.GetDashboard)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimitMiddleware(middleware.APIRate))
		r.Use(middleware.AuthMiddleware(sm))
		r.Get("/admin/api/stats", statsH.GetStats)
	})

	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		middleware.ClearAuthenticated(r.Context(), sm)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimitMiddleware(middleware.APIRate))
		r.Get("/s/{token}", dlH.ServeShare)
		r.Post("/s/{token}", dlH.VerifySharePassword)
		r.Get("/s/{token}/raw", dlH.RawContent)
		r.Get("/s/{token}/download", dlH.DownloadFile)
	})

	r.NotFound(NotFoundHandler)
	r.MethodNotAllowed(MethodNotAllowedHandler)

	srv := httptest.NewServer(r)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	return client, srv.URL, func() { srv.Close(); rdb.Close() }
}

// noRedirectClient returns an http.Client that shares the same cookiejar
// but does NOT follow HTTP redirects (lets us see 303/302 status codes).
func noRedirectClient(jar http.CookieJar) *http.Client {
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// loginAsAdmin bypasses the CSRF login flow by using a test-only endpoint.
// Strategy: cookiejar auto-handles cookies from responses, so subsequent requests
// within the same client will include the session cookie automatically.
func loginAsAdmin(t *testing.T, client *http.Client, baseURL string, sessionCookie *string) {
	t.Helper()

	noRedirect := noRedirectClient(client.Jar)

	if sessionCookie != nil && *sessionCookie != "" {
		// Use previously obtained session token
		req, _ := http.NewRequest("GET", baseURL+"/admin/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: *sessionCookie, Path: "/"})
		resp, err := noRedirect.Do(req)
		if err != nil {
			t.Fatalf("GET /admin/ with session: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		// Session not valid, fall through to re-login
	}

	// Hit the test-login endpoint - cookiejar will capture the session cookie
	resp, err := client.Get(baseURL + "/test-login")
	if err != nil {
		t.Fatalf("GET /test-login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /test-login = %d, want 200", resp.StatusCode)
	}

	// Now try admin with cookies from jar (automatically attached)
	resp, err = noRedirect.Get(baseURL + "/admin/")
	if err != nil {
		t.Fatalf("GET /admin/ after login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cookies := client.Jar.Cookies(mustParseURL(baseURL))
		t.Logf("Cookies in jar: %v", cookies)
		t.Fatalf("GET /admin/ after login = %d, want 200", resp.StatusCode)
	}

	// Extract session cookie value from jar for tests that use manual cookies
	if sessionCookie != nil {
		for _, c := range client.Jar.Cookies(mustParseURL(baseURL)) {
			if c.Name == "session" {
				*sessionCookie = c.Value
				break
			}
		}
	}
}

// mustParseURL is a test helper to parse a URL string without error.
func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

// TestSCSSession verifies SCS session cookie flow works end-to-end.
func TestSCSSession(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()

	// First request: should set a session cookie
	resp, err := client.Get(baseURL + "/test-login")
	if err != nil {
		t.Fatalf("GET /test-login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /test-login = %d, want 200", resp.StatusCode)
	}
	cookies := resp.Cookies()
	t.Logf("Response cookies: %v", cookies)
	_ = baseURL

	// Check cookiejar has the session cookie
	jarCookies := client.Jar.Cookies(mustParseURL(baseURL))
	t.Logf("Jar cookies after first request: %v", jarCookies)

	// Second request: should include the session cookie and see authenticated=true
	noRedirect := noRedirectClient(client.Jar)
	resp, err = noRedirect.Get(baseURL + "/admin/")
	if err != nil {
		t.Fatalf("GET /admin/: %v", err)
	}
	resp.Body.Close()
	t.Logf("/admin/ status: %d (location: %s)", resp.StatusCode, resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/ returned %d, want 200 - session not persisted", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// F1 + F3: Integration tests
// ---------------------------------------------------------------------------

func TestIntegration_LoginPage(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.Get(baseURL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}
}

func TestIntegration_LoginFlow(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)
	t.Logf("session cookie: %s", cookie)
}

func TestIntegration_Dashboard(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)

	req, _ := http.NewRequest("GET", baseURL+"/admin/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("dashboard = %d, want 200. Body: %s", resp.StatusCode, body)
	}
}

func TestIntegration_StatsAPI(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)

	req, _ := http.NewRequest("GET", baseURL+"/admin/api/stats", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stats = %d, want 200", resp.StatusCode)
	}

	var s map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&s)
	if s["total_files"].(float64) != 1 {
		t.Errorf("total_files = %v, want 1", s["total_files"])
	}
	if s["total_shares"].(float64) != 1 {
		t.Errorf("total_shares = %v, want 1", s["total_shares"])
	}
}

func TestIntegration_PublicSharePage(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.Get(baseURL + "/s/testtokn")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("share page = %d, want 200. Body: %s", resp.StatusCode, body)
	}
}

func TestIntegration_PublicShareDownload(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.Get(baseURL + "/s/testtokn/download")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", resp.Header.Get("Content-Disposition"))
	}
}

func TestIntegration_NotFound(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)

	req, _ := http.NewRequest("GET", baseURL+"/nonexistent", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("nonexistent = %d, want 404", resp.StatusCode)
	}
}

func TestIntegration_HealthCheck(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Fatalf("health body = %q, want 'OK'", body)
	}
}

func TestIntegration_Logout(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)

	noRedirect := noRedirectClient(client.Jar)
	req, _ := http.NewRequest("POST", baseURL+"/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
	resp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout = %d, want 303", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/login" {
		t.Fatalf("logout redirect = %q, want /login", resp.Header.Get("Location"))
	}

	// Verify session cleared
	req2, _ := http.NewRequest("GET", baseURL+"/admin/", nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
	resp, err = noRedirect.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("admin after logout = %d, want 303 redirect to login", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// F3: Concurrency check — 10 parallel requests
// ---------------------------------------------------------------------------

func TestIntegration_ConcurrentRequests(t *testing.T) {
	client, baseURL, cleanup := setupTestServer(t)
	defer cleanup()
	var cookie string
	loginAsAdmin(t, client, baseURL, &cookie)

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", baseURL+"/admin/", nil)
			req.AddCookie(&http.Cookie{Name: "session", Value: cookie, Path: "/"})
			resp, err := client.Do(req)
			if err != nil {
				errs <- fmt.Errorf("request: %w", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("status %d", resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	close(errs)

	var failures []string
	for e := range errs {
		failures = append(failures, e.Error())
	}
	if len(failures) > 0 {
		t.Errorf("%d concurrent failures:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}
