package admin

import (
	"bytes"
	"html/template"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func repoRootDirForTests(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// filename is .../internal/admin/templates_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func TestTokensListTemplate_TotalResponsesServedIsSum(t *testing.T) {
	templateDir := filepath.Join(repoRootDirForTests(t), "web", "templates")

	s := &Server{assetVersion: "test"}
	tmpl := template.Must(template.New("").Funcs(s.templateFuncs()).ParseGlob(filepath.Join(templateDir, "*.html")))
	tmpl = template.Must(tmpl.ParseGlob(filepath.Join(templateDir, "*", "*.html")))

	upstream := 12
	cacheHits := 34
	data := map[string]any{
		"title":        "Tokens",
		"active":       "tokens",
		"tokens":       []Token{{ID: "tok-1", Token: "secret", ProjectID: "1", IsActive: true, RequestCount: upstream, CacheHitCount: cacheHits, CreatedAt: time.Now()}},
		"pagination":   &Pagination{Page: 1, PageSize: 10, TotalItems: 1, TotalPages: 1},
		"projectId":    "",
		"projectNames": map[string]string{"1": "Test Project"},
		"now":          time.Now(),
		"currentTime":  time.Now(),
	}

	var buf bytes.Buffer
	require.NoError(t, tmpl.ExecuteTemplate(&buf, "tokens/list.html", data))

	expectedTotal := strconv.Itoa(upstream + cacheHits)
	require.Contains(t, buf.String(), expectedTotal)
}
