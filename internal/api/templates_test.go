package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestProjectTemplateCompatFields(t *testing.T) {
	templateID := uuid.New()
	project := model.Project{
		ID:         uuid.New(),
		Name:       "Order from Project",
		TemplateID: &templateID,
		Source:     "manual",
	}

	if project.TemplateID == nil {
		t.Fatal("expected template_id to be set")
	}
	if *project.TemplateID != templateID {
		t.Errorf("expected template ID %s, got %s", templateID, *project.TemplateID)
	}
}

func TestParseUUID_Templates(t *testing.T) {
	t.Run("valid UUID", func(t *testing.T) {
		r := chi.NewRouter()
		var parsedID uuid.UUID

		r.Get("/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := parseUUID(r, "id")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			parsedID = id
			w.WriteHeader(http.StatusOK)
		})

		testID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/templates/"+testID.String(), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
		if parsedID != testID {
			t.Errorf("expected ID %s, got %s", testID, parsedID)
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		r := chi.NewRouter()

		r.Get("/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
			_, err := parseUUID(r, "id")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/templates/invalid-uuid", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid UUID, got %d", rr.Code)
		}
	})
}
