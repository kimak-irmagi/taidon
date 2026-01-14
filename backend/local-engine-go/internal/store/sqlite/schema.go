package sqlite

import _ "embed"

//go:embed schema.sql
var schemaSQL string

func SchemaSQL() string {
	return schemaSQL
}
