package migration

import (
	"strings"
	"testing"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

func TestSourceForDefaultSources(t *testing.T) {
	t.Parallel()

	registry := testRegistry()
	tests := []string{"", "gateway", " GATEWAY ", "user", " USER "}
	for _, tt := range tests {
		source, err := registry.SourceFor(tt)
		if err != nil {
			t.Fatalf("SourceFor(%q) returned error: %v", tt, err)
		}
		if source.Models == nil {
			t.Fatalf("SourceFor(%q) returned nil Models", tt)
		}
		if source.JoinTables == nil {
			t.Fatalf("SourceFor(%q) returned nil JoinTables", tt)
		}
	}
}

func TestSourceForUnknownService(t *testing.T) {
	t.Parallel()

	_, err := testRegistry().SourceFor("order")
	if err == nil {
		t.Fatal("SourceFor(order) expected error")
	}
	if !strings.Contains(err.Error(), "available: gateway, idgen, user") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceNamesStable(t *testing.T) {
	t.Parallel()

	names := testRegistry().ServiceNames()
	if got, want := strings.Join(names, ","), "gateway,idgen,user"; got != want {
		t.Fatalf("ServiceNames() = %q, want %q", got, want)
	}
}

func testRegistry() Registry {
	return Registry{
		"gateway": {
			Name:       "gateway",
			Models:     func() []any { return []any{} },
			JoinTables: func() []mysql.MigrationJoinTable { return []mysql.MigrationJoinTable{} },
		},
		"user": {
			Name:       "user",
			Models:     func() []any { return []any{} },
			JoinTables: func() []mysql.MigrationJoinTable { return []mysql.MigrationJoinTable{} },
		},
		"idgen": {
			Name:       "idgen",
			Models:     func() []any { return []any{} },
			JoinTables: func() []mysql.MigrationJoinTable { return []mysql.MigrationJoinTable{} },
		},
	}
}
