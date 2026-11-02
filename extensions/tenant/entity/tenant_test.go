package entity_test

import (
	"regexp"
	"strings"
	"sync"
	"testing"

	"gitlab.com/skyrix-lib/framework/extensions/tenant/entity"

	"gorm.io/gorm/schema"
)

var (
	identifier  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	sqlKeywords = map[string]bool{"is": true, "not": true, "null": true, "and": true, "or": true, "true": true, "false": true, "in": true}
)

// TestIndexesReferenceExistingColumns guards against index tags whose
// columns or WHERE clauses name columns the model does not have; AutoMigrate
// fails on such indexes.
func TestIndexesReferenceExistingColumns(t *testing.T) {
	for _, model := range []any{&entity.Tenant{}} {
		s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		columns := map[string]bool{}
		for _, name := range s.DBNames {
			columns[name] = true
		}
		for _, index := range s.ParseIndexes() {
			for _, option := range index.Fields {
				if !columns[option.DBName] {
					t.Errorf("%s: index %s uses unknown column %q", s.Table, index.Name, option.DBName)
				}
			}
			for _, word := range identifier.FindAllString(index.Where, -1) {
				if !sqlKeywords[strings.ToLower(word)] && !columns[word] {
					t.Errorf("%s: index %s WHERE %q references unknown column %q", s.Table, index.Name, index.Where, word)
				}
			}
		}
	}
}
