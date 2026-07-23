//go:build with_etl || with_all

package main

import (
	// Register the ETL Extension Component Library
	// Use `go build -tags with_etl .` to include the ETL extension components in the executable
	_ "github.com/rulego/rulego-components-etl/endpoint/mysql_cdc"
)
