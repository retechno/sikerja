package handler_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/retechno/sikerja/backend/internal/app"
	"github.com/retechno/sikerja/backend/internal/handler"
	"github.com/retechno/sikerja/backend/internal/storage"
	"github.com/retechno/sikerja/backend/internal/testutil"
)

var ginParam = regexp.MustCompile(`:(\w+)`)

// Kontrak docs/api.yaml harus mendeskripsikan setiap route /api/v1 (dan sebaliknya).
func TestApiYamlMatchesRouter(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/api.yaml")
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))

	documented := map[string]bool{}
	for p, methods := range spec.Paths {
		for m := range methods {
			switch m {
			case "get", "post", "put", "delete", "patch":
				documented[strings.ToUpper(m)+" /api/v1"+p] = true
			}
		}
	}

	w := testutil.NewWorld(t)
	files, _ := storage.NewLocal(w.Cfg.StorageDir)
	router := handler.NewRouter(app.NewDeps(w.Cfg, zerolog.Nop(), w.DB, w.Rdb, w.Mailer, files))
	implemented := map[string]bool{}
	for _, r := range router.Routes() {
		if strings.HasPrefix(r.Path, "/api/v1") {
			implemented[r.Method+" "+ginParam.ReplaceAllString(r.Path, "{$1}")] = true
		}
	}

	var missingDoc, missingImpl []string
	for k := range implemented {
		if !documented[k] {
			missingDoc = append(missingDoc, k)
		}
	}
	for k := range documented {
		if !implemented[k] {
			missingImpl = append(missingImpl, k)
		}
	}
	sort.Strings(missingDoc)
	sort.Strings(missingImpl)
	assert.Empty(t, missingDoc, "route belum ada di docs/api.yaml")
	assert.Empty(t, missingImpl, "ada di docs/api.yaml tetapi tidak ada route-nya")
}
