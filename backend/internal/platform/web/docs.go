package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
)

// Bản swagger-ui-dist trên CDN; đổi bản thì đổi ở đây
const swaggerUIVersion = "5.33.1"

// APIDoc là spec OpenAPI của một module, hiện trên trang tài liệu. Spec lấy từ
// api.GetSwagger() của module: bản nhúng đã resolve ref sang api/common.yaml.
type APIDoc struct {
	Name string // dùng trong URL: chữ thường, số, gạch ngang
	Spec *openapi3.T
}

var docNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// MountDocs gắn trang Swagger UI ở GET /api/docs và spec JSON của từng module ở
// GET /api/docs/<name>.json. Cùng origin với API nên "Try it out" gọi thẳng
// được: đăng nhập, dán access_token vào Authorize; cookie refresh cũng đi kèm.
//
// Trang công khai, không qua auth: chỉ bật khi dev (HTTP_API_DOCS). JS của
// Swagger UI tải từ CDN, nên máy xem trang cần internet.
func MountDocs(r chi.Router, docs ...APIDoc) error {
	if len(docs) == 0 {
		return errors.New("web: MountDocs needs at least one spec")
	}
	type entry struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	entries := make([]entry, 0, len(docs))
	seen := map[string]bool{}
	for _, d := range docs {
		switch {
		case !docNamePattern.MatchString(d.Name):
			return fmt.Errorf("web: API doc name %q: use lowercase letters, digits and dashes", d.Name)
		case seen[d.Name]:
			return fmt.Errorf("web: API doc %q registered twice", d.Name)
		case d.Spec == nil:
			return fmt.Errorf("web: API doc %q has no spec", d.Name)
		}
		seen[d.Name] = true
		// Marshal một lần lúc gắn: spec không đổi khi server chạy
		body, err := json.Marshal(d.Spec)
		if err != nil {
			return fmt.Errorf("web: API doc %q: %w", d.Name, err)
		}
		path := "/api/docs/" + d.Name + ".json"
		r.Get(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(body)
		})
		entries = append(entries, entry{URL: path, Name: d.Name})
	}

	var page bytes.Buffer
	if err := docsPage.Execute(&page, struct {
		Version string
		URLs    []entry
	}{swaggerUIVersion, entries}); err != nil {
		return fmt.Errorf("web: render API docs page: %w", err)
	}
	html := page.Bytes()
	r.Get("/api/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(html)
	})
	return nil
}

// html/template tự mã hoá URLs thành literal JS an toàn trong <script>
var docsPage = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>StoreIt API</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@{{.Version}}/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@{{.Version}}/swagger-ui-bundle.js"></script>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@{{.Version}}/swagger-ui-standalone-preset.js"></script>
<script>
window.ui = SwaggerUIBundle({
  urls: {{.URLs}},
  dom_id: "#swagger-ui",
  presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
  layout: "StandaloneLayout",
  // Giữ access token đã dán vào Authorize khi tải lại trang
  persistAuthorization: true,
  displayRequestDuration: true,
});
</script>
</body>
</html>
`))
