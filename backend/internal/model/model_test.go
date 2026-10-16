package model_test

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"

	"github.com/retechno/sikerja/backend/internal/model"
	"github.com/retechno/sikerja/backend/migrations"
)

var createTable = regexp.MustCompile(`(?s)CREATE TABLE (\w+) \((.*?)\n\) ENGINE`)

func migrationColumns(t *testing.T) map[string][]string {
	t.Helper()
	files, err := fs.Glob(migrations.FS, "*.up.sql")
	require.NoError(t, err)
	var raw []byte
	for _, f := range files {
		b, err := migrations.FS.ReadFile(f)
		require.NoError(t, err)
		raw = append(raw, b...)
	}
	out := map[string][]string{}
	for _, m := range createTable.FindAllStringSubmatch(string(raw), -1) {
		var cols []string
		for _, line := range strings.Split(m[2], "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			first := strings.ToUpper(strings.Fields(line)[0])
			switch first {
			case "PRIMARY", "UNIQUE", "KEY", "CONSTRAINT":
				continue
			}
			cols = append(cols, strings.Fields(line)[0])
		}
		sort.Strings(cols)
		out[m[1]] = cols
	}
	return out
}

// Kolom di model GORM harus sama persis dengan kolom di migrasi SQL produksi.
func TestModelsMatchMigration(t *testing.T) {
	mig := migrationColumns(t)
	for _, m := range model.All() {
		s, err := schema.Parse(m, &sync.Map{}, schema.NamingStrategy{})
		require.NoError(t, err)
		var cols []string
		for _, f := range s.Fields {
			if f.DBName != "" {
				cols = append(cols, f.DBName)
			}
		}
		sort.Strings(cols)
		want, ok := mig[s.Table]
		if assert.True(t, ok, "tabel %s tidak ada di migrasi", s.Table) {
			assert.Equal(t, want, cols, "kolom tabel %s", s.Table)
		}
	}
}
