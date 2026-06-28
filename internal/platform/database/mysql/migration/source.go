package migration

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/gorm"
)

// Source describes one Atlas GORM schema source.
type Source struct {
	Name       string
	Models     func() []any
	JoinTables func() []mysql.MigrationJoinTable
}

// Registry maps service names to Atlas GORM schema sources.
type Registry map[string]Source

// GormConfig returns the shared GORM configuration used by Atlas schema loading.
func GormConfig() *gorm.Config {
	return mysql.GormConfig()
}

// SourceFor returns the schema source registered for service.
func (r Registry) SourceFor(service string) (Source, error) {
	name := NormalizeService(service)
	source, ok := r[name]
	if !ok {
		return Source{}, fmt.Errorf("unknown atlas migration service %q, available: %s", service, strings.Join(r.ServiceNames(), ", "))
	}
	return source, nil
}

// NormalizeService maps empty service names to the gateway aggregation service source.
func NormalizeService(service string) string {
	name := strings.TrimSpace(strings.ToLower(service))
	if name == "" {
		return "gateway"
	}
	return name
}

// ServiceNames returns registered service names in stable order.
func (r Registry) ServiceNames() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
