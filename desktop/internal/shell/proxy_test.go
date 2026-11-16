package desktop

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testHost(t *testing.T) (*Host, *httptest.Server) {
	t.Helper()
	h := &Host{Config: DefaultConfig(), Secret: "session-secret", Bootstrap: "one-shot", Assets: fstest.MapFS{"index.html": {Data: []byte("<html><head></head><body>bundled</body></html>")}}}
	s := httptest.NewServer(h.Handler())
	h.Origin = s.URL
	t.Cleanup(s.Close)
	return h, s
}
func req(t *testing.T, s *httptest.Server, method, path, origin, body string) *http.Response {
	t.Helper()
	r, _ := http.NewRequest(method, s.URL+path, strings.NewReader(body))
	r.AddCookie(&http.Cookie{Name: "pf_desktop", Value: "session-secret"})
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	client := *s.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}
func TestGatewayOriginAndHost(t *testing.T) {
	_, s := testHost(t)
	for _, path := range []string{"/api/test", "/ws", "/desktop/notify"} {
		if res := req(t, s, "POST", path, "https://evil.example", `{}`); res.StatusCode != 403 {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
	r, _ := http.NewRequest("GET", s.URL+"/", nil)
	r.Host = "evil.example"
	r.AddCookie(&http.Cookie{Name: "pf_desktop", Value: "session-secret"})
	res, err := s.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("forged host: %d", res.StatusCode)
	}
	if res := req(t, s, "POST", "/desktop/notify", "", `{}`); res.StatusCode != 403 {
		t.Fatal("native mutation without origin accepted")
	}
}
func TestBootstrapSingleUse(t *testing.T) {
	_, s := testHost(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for i := 0; i < 2; i++ {
		res, err := client.Get(s.URL + "/desktop/bootstrap?key=one-shot")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 303
		if i == 1 {
			want = 403
		}
		if res.StatusCode != want {
			t.Fatalf("bootstrap %d: %d", i, res.StatusCode)
		}
		if i == 0 {
			c := res.Cookies()[0]
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Fatal("unsafe cookie")
			}
		}
	}
}
func TestProxyScrubsSecretsAndRejectsRedirect(t *testing.T) {
	var auth, cookie string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
		w.Header().Set("Set-Cookie", "pf_desktop=upstream")
		if r.URL.Path == "/api/redirect" {
			http.Redirect(w, r, "https://evil.example", 302)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<script>evil()</script>")
	}))
	defer upstream.Close()
	h, s := testHost(t)
	h.Target, _ = url.Parse(upstream.URL)
	h.Token = "configured-token"
	res := req(t, s, "GET", "/api/test", s.URL, "")
	if auth != "Bearer configured-token" || cookie != "" {
		t.Fatalf("headers: %q %q", auth, cookie)
	}
	if res.Header.Get("Set-Cookie") != "" || res.Header.Get("Content-Disposition") != "attachment" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("upstream active content/cookies not isolated")
	}
	if res := req(t, s, "GET", "/api/redirect", s.URL, ""); res.StatusCode != 502 {
		t.Fatalf("redirect %d", res.StatusCode)
	}
	res = req(t, s, "GET", "/", s.URL, "")
	b, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(b), "bundled") || strings.Contains(string(b), "evil()") {
		t.Fatal("remote UI served")
	}
}
func TestProxyRedirectPolicy(t *testing.T) {
	var gatewayOrigin string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/relative":
			http.Redirect(w, r, "/settings/integrations?shopify=connected", http.StatusFound)
		case "/api/absolute":
			http.Redirect(w, r, gatewayOrigin+"/settings?etsy=connected", http.StatusFound)
		case "/api/hostile":
			http.Redirect(w, r, "https://evil.example/steal", http.StatusFound)
		case "/api/protocol-relative":
			http.Redirect(w, r, "//evil.example/steal", http.StatusFound)
		case "/api/injected":
			http.Redirect(w, r, "/settings\r\nX-Injected: 1", http.StatusFound)
		default:
			http.Error(w, "unexpected", 404)
		}
	}))
	defer upstream.Close()
	h, s := testHost(t)
	gatewayOrigin = s.URL
	h.Target, _ = url.Parse(upstream.URL)
	h.Token = "configured-token"
	res := req(t, s, "GET", "/api/relative", s.URL, "")
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/settings/integrations?shopify=connected" {
		t.Fatalf("relative callback redirect: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	res = req(t, s, "GET", "/api/absolute", s.URL, "")
	if got := res.Header.Get("Location"); got != "/settings?etsy=connected" {
		t.Fatalf("same-origin redirect not collapsed: %q", got)
	}
	for _, path := range []string{"/api/hostile", "/api/protocol-relative"} {
		if res := req(t, s, "GET", path, s.URL, ""); res.StatusCode != 502 {
			t.Fatalf("%s refused with %d", path, res.StatusCode)
		}
	}
	// Go's header writer replaces control characters, so a CRLF attempt
	// cannot smuggle a second response header; it must not become a usable
	// location either and must not surface as a separate header.
	res = req(t, s, "GET", "/api/injected", s.URL, "")
	if res.Header.Get("X-Injected") != "" || strings.ContainsAny(res.Header.Get("Location"), "\r\n") {
		t.Fatalf("header injection survived: %q %q", res.Header.Get("X-Injected"), res.Header.Get("Location"))
	}
}
func TestRemoteValidation(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com/path", "https://example.com?token=x", "https://example.com/#x", "javascript:alert(1)", "http://localhost:8084"} {
		if _, err := ValidateRemote(raw, true); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if _, err := ValidateRemote("http://127.0.0.1:8084", false); err == nil {
		t.Fatal("implicit insecure HTTP")
	}
	for _, raw := range []string{"https://example.com", "http://127.0.0.1:8084", "http://[::1]:8084"} {
		if _, err := ValidateRemote(raw, true); err != nil {
			t.Fatal(err)
		}
	}
}
func TestPrivatePersistence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	c := DefaultConfig()
	c.RemoteToken = "secret"
	if err := SaveConfig(dir, c); err != nil {
		t.Fatal(err)
	}
	a, err := EncryptionKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncryptionKey(dir)
	if err != nil || a != b || len(a) != 64 {
		t.Fatal("key not persistent")
	}
	for _, name := range []string{"config.json", "encryption.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("%s mode %v", name, info.Mode())
		}
	}
	h, s := testHost(t)
	h.Config = c
	res := req(t, s, "GET", "/desktop/config", s.URL, "")
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), "secret") {
		t.Fatal("config leaks token")
	}
}
