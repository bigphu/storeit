package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
)

func docsSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromData([]byte(testSpec))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func docsRouter(t *testing.T, docs ...APIDoc) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	if err := MountDocs(r, docs...); err != nil {
		t.Fatal(err)
	}
	return r
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestDocs_PageListsEverySpec(t *testing.T) {
	h := docsRouter(t, APIDoc{Name: "identity", Spec: docsSpec(t)}, APIDoc{Name: "inventory", Spec: docsSpec(t)})
	rec := get(h, "/api/docs")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("page: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{
		"swagger-ui-dist@" + swaggerUIVersion + "/swagger-ui-bundle.js",
		`/api/docs/identity.json`,
		`/api/docs/inventory.json`,
		"persistAuthorization",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// Trang dev, không để proxy hay trình duyệt cache bản cũ
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestDocs_ServesSpecAsJSON(t *testing.T) {
	h := docsRouter(t, APIDoc{Name: "identity", Spec: docsSpec(t)})
	rec := get(h, "/api/docs/identity.json")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("spec: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got struct {
		OpenAPI string                     `json:"openapi"`
		Servers []struct{ URL string }     `json:"servers"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.OpenAPI != "3.0.3" || len(got.Servers) != 1 || got.Servers[0].URL != "/api/v1" || got.Paths["/things"] == nil {
		t.Errorf("spec = %+v", got)
	}
	if rec := get(h, "/api/docs/unknown.json"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown spec: %d, want 404", rec.Code)
	}
}

func TestDocs_RejectsBadSetup(t *testing.T) {
	spec := docsSpec(t)
	for name, docs := range map[string][]APIDoc{
		"no specs":      nil,
		"duplicate":     {{Name: "identity", Spec: spec}, {Name: "identity", Spec: spec}},
		"bad name":      {{Name: "Identity API", Spec: spec}},
		"empty name":    {{Name: "", Spec: spec}},
		"missing spec":  {{Name: "identity"}},
		"path in name":  {{Name: "../x", Spec: spec}},
		"upper in name": {{Name: "Identity", Spec: spec}},
	} {
		if err := MountDocs(chi.NewRouter(), docs...); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
