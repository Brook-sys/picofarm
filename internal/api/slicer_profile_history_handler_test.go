package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
)

func TestSlicerEditorValidationHandler(t *testing.T) {
	h := NewSlicerHandler(&service.SlicerService{})
	cases := []struct {
		body     string
		status   int
		contains string
	}{
		{`{"category":"presets","name":"q","json":"null"}`, 200, `"valid":false`},
		{`{"category":"presets","name":"q","json":"[]"}`, 200, `"valid":false`},
		{`{"category":"presets","name":"q","json":"{\"layer_height\":\"0.2\"}"}`, 200, `"valid":true`},
		{`{"category":"presets"} {}`, 400, "one JSON object"},
		{`{"json":"` + strings.Repeat("x", 2*service.SlicerProfileJSONLimit) + `"}`, 400, "oversized"},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		h.ValidateProfile(w, r)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}
