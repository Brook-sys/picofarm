package updates

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, body             string
		status                 int
		wantError, wantRelease bool
	}{
		{"stable", `{"tag_name":"v1.2.3","html_url":"https://github.com/Brook-sys/picofarm/releases/tag/v1.2.3"}`, 200, false, true},
		{"absent", ``, 404, false, false},
		{"rate limited", ``, 403, true, false},
		{"foreign", `{"tag_name":"v1","html_url":"https://example.com/app.exe"}`, 200, true, false},
		{"draft", `{"tag_name":"v1","html_url":"https://github.com/Brook-sys/picofarm/releases/tag/v1","draft":true}`, 200, true, false},
		{"invalid", `{`, 200, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("unexpected credentials")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			result, err := check(context.Background(), srv.Client(), srv.URL)
			if (err != nil) != tc.wantError || (result != nil) != tc.wantRelease {
				t.Fatalf("result=%v err=%v", result, err)
			}
		})
	}
}
