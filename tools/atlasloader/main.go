package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"ariga.io/atlas-provider-gorm/gormschema"
	gatewayschema "github.com/egoadmin/egoadmin/internal/app/gateway/schema"
	idgenschema "github.com/egoadmin/egoadmin/internal/app/idgen/schema"
	userschema "github.com/egoadmin/egoadmin/internal/app/user/schema"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql/migration"
)

func main() {
	service := flag.String("service", "gateway", "migration schema source service name")
	dialect := flag.String("dialect", "mysql", "gorm schema dialect: mysql, postgres, sqlite, or sqlserver")
	listServices := flag.Bool("list-services", false, "list registered migration schema sources")
	flag.Parse()

	registry := schemaRegistry()
	if *listServices {
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(registry.ServiceNames(), "\n"))
		return
	}

	source, err := registry.SourceFor(*service)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	normalizedDialect, err := normalizeDialect(*dialect)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}

	options := []gormschema.Option{
		gormschema.WithConfig(migration.GormConfig()),
	}
	if normalizedDialect == "sqlserver" {
		options = append(options, gormschema.WithStmtDelimiter("\nGO"))
	}
	for _, joinTable := range source.JoinTables() {
		options = append(options, gormschema.WithJoinTable(joinTable.Model, joinTable.Field, joinTable.Table))
	}

	stmts, err := gormschema.New(normalizedDialect, options...).Load(source.Models()...)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	_, _ = io.WriteString(os.Stdout, stmts)
}

func schemaRegistry() migration.Registry {
	return migration.Registry{
		"gateway": {
			Name:       "gateway",
			Models:     gatewayschema.MigrationModels,
			JoinTables: gatewayschema.MigrationJoinTables,
		},
		"user": {
			Name:       "user",
			Models:     userschema.MigrationModels,
			JoinTables: userschema.MigrationJoinTables,
		},
		"idgen": {
			Name:       "idgen",
			Models:     idgenschema.MigrationModels,
			JoinTables: idgenschema.MigrationJoinTables,
		},
	}
}

func normalizeDialect(dialect string) (string, error) {
	name := strings.TrimSpace(strings.ToLower(dialect))
	if name == "" {
		return "mysql", nil
	}
	switch name {
	case "mysql", "postgres", "sqlite", "sqlserver":
		return name, nil
	default:
		return "", fmt.Errorf("unknown atlas schema dialect %q, available: mysql, postgres, sqlite, sqlserver", dialect)
	}
}
