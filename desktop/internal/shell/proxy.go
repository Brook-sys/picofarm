package desktop

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Notification struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Category string `json:"category"`
}
type Callbacks struct {
	OpenExternal func(string) error
	Notify       func(Notification) error
	Save         func(Config) error
}
type Host struct {
	mu        sync.RWMutex
	Config    Config
	Assets    fs.FS
	Target    *url.URL
	Token     string
	Secret    string
	Bootstrap string
	Origin    string
	Callbacks Callbacks
	transport http.RoundTripper
}

func (h *Host) Handler() http.Handler {
	if h.transport == nil {
		h.transport = &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second}
	}
	return http.HandlerFunc(h.serve)
}
func (h *Host) CloseIdleConnections() {
	if t, ok := h.transport.(interface{ CloseIdleConnections() }); ok {
		t.CloseIdleConnections()
	}
}
func (h *Host) serve(w http.ResponseWriter, r *http.Request) {
	if r.Host != strings.TrimPrefix(h.Origin, "http://") {
		http.Error(w, "invalid host", 403)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/desktop/bootstrap" && r.Method == "GET" {
		h.mu.Lock()
		valid := h.Bootstrap != "" && equal(r.URL.Query().Get("key"), h.Bootstrap)
		if valid {
			h.Bootstrap = ""
		}
		h.mu.Unlock()
		if !valid {
			http.Error(w, "invalid bootstrap", 403)
			return
		}
		// Secure cannot be used on a plain HTTP loopback origin across all WebViews.
		http.SetCookie(w, &http.Cookie{Name: "pf_desktop", Value: h.Secret, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	c, err := r.Cookie("pf_desktop")
	if err != nil || !equal(c.Value, h.Secret) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != h.Origin {
		http.Error(w, "invalid origin", 403)
		return
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		http.Error(w, "cross-site request", 403)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && r.Header.Get("Origin") != h.Origin {
		http.Error(w, "origin required", 403)
		return
	}
	if r.URL.Path == "/desktop/setup" && r.Method == "GET" {
		h.setup(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/desktop/") {
		h.desktop(w, r)
		return
	}
	if r.URL.Path == "/ws" || r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/uploads/") {
		if h.Target == nil {
			http.Error(w, "backend unavailable", 503)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(h.Target)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.Host = h.Target.Host
			req.Header.Del("Cookie")
			req.Header.Del("Authorization")
			req.Header.Del("X-API-Key")
			req.Header.Del("Origin")
			req.Header.Del("Referer")
			req.Header.Del("Forwarded")
			req.Header.Del("X-Forwarded-Host")
			req.Header.Del("X-Forwarded-Proto")
			req.Header.Set("Authorization", "Bearer "+h.Token)
			q := req.URL.Query()
			q.Del("token")
			q.Del("api_token")
			if req.URL.Path == "/ws" {
				q.Set("token", h.Token)
			}
			req.URL.RawQuery = q.Encode()
		}
		proxy.ModifyResponse = func(res *http.Response) error {
			res.Header.Del("Set-Cookie")
			res.Header.Del("Access-Control-Allow-Origin")
			// No upstream document may execute scripts or acquire shell privileges.
			res.Header.Set("Content-Security-Policy", "sandbox; default-src 'none'")
			res.Header.Set("X-Content-Type-Options", "nosniff")
			// Integration callbacks (Shopify, Etsy) redirect back to app routes
			// such as /settings?..., and those must survive the gateway. Only
			// redirects that stay inside this same-origin shell are allowed.
			if loc := res.Header.Get("Location"); loc != "" {
				safe, err := sameOriginLocation(loc, h.Origin)
				if err != nil {
					return err
				}
				if safe != loc {
					res.Header.Set("Location", safe)
				}
			}
			ct := res.Header.Get("Content-Type")
			if strings.Contains(ct, "html") || strings.Contains(ct, "javascript") || strings.Contains(ct, "svg") {
				res.Header.Set("Content-Disposition", "attachment")
			}
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "backend connection failed", 502)
		}
		proxy.Transport = h.transport
		proxy.ServeHTTP(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; media-src 'self' blob: https: http:; connect-src 'self' ws: https: http:; worker-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-src 'none'; form-action 'self'")
	if r.URL.Path == "/desktop-bridge.js" {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, bridgeJS)
		return
	}
	if h.Assets == nil {
		http.Error(w, "frontend assets not staged", 503)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	b, err := fs.ReadFile(h.Assets, name)
	if err != nil {
		name = "index.html"
		b, err = fs.ReadFile(h.Assets, name)
	}
	if err != nil {
		http.Error(w, "frontend assets not staged", 503)
		return
	}
	if name == "index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, strings.Replace(string(b), "<head>", "<head><script src=\"/desktop-bridge.js\"></script>", 1))
		return
	}
	http.FileServer(http.FS(h.Assets)).ServeHTTP(w, r)
}
func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// sameOriginLocation permits only redirects that resolve inside the gateway
// origin. A relative app path passes through unchanged; an absolute URL that
// points at the gateway itself collapses to its path so the webview is never
// shown a foreign host. Protocol-relative and absolute foreign targets are
// refused, which keeps an upstream redirect from exfiltrating the shell
// session to another origin.
func sameOriginLocation(location, origin string) (string, error) {
	if strings.ContainsAny(location, "\r\n") {
		return "", fmt.Errorf("upstream redirects are not allowed")
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("upstream redirects are not allowed")
	}
	if u.Scheme == "" && u.Host == "" && !strings.HasPrefix(location, "//") {
		return location, nil
	}
	want, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("upstream redirects are not allowed")
	}
	if u.Scheme == want.Scheme && u.Host == want.Host && u.User == nil {
		return u.RequestURI(), nil
	}
	return "", fmt.Errorf("upstream redirects are not allowed")
}
func (h *Host) desktop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" && !(r.Method == "GET" && r.URL.Path == "/desktop/config") {
		http.Error(w, "method not allowed", 405)
		return
	}
	// All native mutations require an exact origin, even if a browser omits it.
	if r.Method == "POST" && r.Header.Get("Origin") != h.Origin {
		http.Error(w, "origin required", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/desktop/config":
		if r.Method == "GET" {
			h.mu.RLock()
			c := h.Config
			h.mu.RUnlock()
			c.RemoteToken = ""
			json.NewEncoder(w).Encode(c)
			return
		}
		var c Config
		if json.NewDecoder(r.Body).Decode(&c) != nil {
			http.Error(w, "invalid config", 400)
			return
		}
		h.mu.RLock()
		if c.RemoteToken == "" && c.RemoteURL == h.Config.RemoteURL {
			c.RemoteToken = h.Config.RemoteToken
		}
		h.mu.RUnlock()
		if err := c.Validate(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if h.Callbacks.Save == nil {
			http.Error(w, "configuration unavailable", 503)
			return
		}
		if err := h.Callbacks.Save(c); err != nil {
			http.Error(w, "could not save configuration", 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"restartRequired": true})
	case "/desktop/open-external":
		var v struct {
			URL string `json:"url"`
		}
		if json.NewDecoder(r.Body).Decode(&v) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		u, err := url.Parse(v.URL)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || strings.ContainsAny(v.URL, "\r\n") {
			http.Error(w, "only HTTP(S) links allowed", 400)
			return
		}
		if h.Callbacks.OpenExternal == nil {
			http.Error(w, "external browser unavailable", 503)
			return
		}
		if err = h.Callbacks.OpenExternal(v.URL); err != nil {
			http.Error(w, "could not open browser", 500)
			return
		}
		fmt.Fprint(w, `{}`)
	case "/desktop/notify":
		var n Notification
		if json.NewDecoder(r.Body).Decode(&n) != nil || len(n.Title) > 200 || len(n.Body) > 2000 {
			http.Error(w, "invalid notification", 400)
			return
		}
		h.mu.RLock()
		enabled := h.Config.Notifications[n.Category]
		h.mu.RUnlock()
		if !enabled {
			json.NewEncoder(w).Encode(map[string]bool{"suppressed": true})
			return
		}
		if h.Callbacks.Notify == nil {
			http.Error(w, "native notifications unavailable", 503)
			return
		}
		if err := h.Callbacks.Notify(n); err != nil {
			http.Error(w, "notification delivery failed", 503)
			return
		}
		fmt.Fprint(w, `{}`)
	default:
		http.NotFound(w, r)
	}
}

const bridgeJS = `(()=>{const post=async(path,data)=>{const r=await fetch('/desktop/'+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});if(!r.ok)throw new Error(await r.text());return r.json()};Object.defineProperty(window,'__PICOFARM_DESKTOP__',{value:Object.freeze({apiBaseURL:location.origin,wsBaseURL:location.origin+'/ws',openExternal:url=>post('open-external',{url}),notify:n=>post('notify',{title:n.title,body:n.body,category:({'picofarm-finished':'finished','picofarm-failed':'failed','picofarm-critical':'critical'})[n.tag]||n.category})}),writable:false});})();`
