package dao

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const defaultSchema = "spendit"

var (
	schemaMu      sync.RWMutex
	schemaName    = defaultSchema
	schemaPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func SetSchema(schema string) error {
	trimmed := strings.TrimSpace(schema)
	if trimmed == "" {
		trimmed = defaultSchema
	}
	if !schemaPattern.MatchString(trimmed) {
		return fmt.Errorf("invalid database schema %q", schema)
	}

	schemaMu.Lock()
	schemaName = strings.ToLower(trimmed)
	schemaMu.Unlock()

	return nil
}

func Schema() string {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	return schemaName
}

func QualifiedTable(table string) string {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	return schemaName + "." + table
}
