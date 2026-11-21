package api

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
)

func TestSlicerOperationPreservesUpstreamStatusAndMessage(t *testing.T) {
	for _, status := range []int{400, 409, 422, 500, 503} {
		w := httptest.NewRecorder()
		respondSlicerOperationError(w, &service.SlicerUpstreamError{StatusCode: status, Message: "requested quantity 12, but produced 4; reduce quantity and retry"})
		if w.Code != status || !strings.Contains(w.Body.String(), "reduce quantity and retry") {
			t.Fatalf("%d: %d %s", status, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	respondSlicerOperationError(w, errors.New("arrange must be true"))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
