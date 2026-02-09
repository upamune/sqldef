package duckdb

import (
	"database/sql"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/sqldef/sqldef/v3/database"
)

type DuckDBDatabase struct {
	config          database.Config
	db              *sql.DB
	generatorConfig database.GeneratorConfig
}

func NewDatabase(config database.Config) (database.Database, error) {
	db, err := sql.Open("duckdb", config.DbName)
	if err != nil {
		return nil, err
	}

	return &DuckDBDatabase{
		db:     db,
		config: config,
	}, nil
}

func (d *DuckDBDatabase) ExportDDLs() (string, error) {
	var ddls []string

	tableNames, err := d.tableNames()
	if err != nil {
		return "", err
	}
	for _, tableName := range tableNames {
		ddl, err := d.exportTableDDL(tableName)
		if err != nil {
			return "", err
		}
		ddls = append(ddls, ddl)
	}

	viewDDLs, err := d.views()
	if err != nil {
		return "", err
	}
	ddls = append(ddls, viewDDLs...)

	indexDDLs, err := d.indexes()
	if err != nil {
		return "", err
	}
	ddls = append(ddls, indexDDLs...)

	return strings.Join(ddls, "\n\n"), nil
}

func (d *DuckDBDatabase) tableNames() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT tbl_name
		FROM sqlite_master
		WHERE type = 'table' AND tbl_name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (d *DuckDBDatabase) exportTableDDL(table string) (string, error) {
	const query = `
		SELECT sql
		FROM sqlite_master
		WHERE tbl_name = ? AND type = 'table'
	`
	var ddl string
	err := d.db.QueryRow(query, table).Scan(&ddl)
	return ddl + ";", err
}

func (d *DuckDBDatabase) views() ([]string, error) {
	var ddls []string
	const query = `
		SELECT sql
		FROM sqlite_master
		WHERE type = 'view'
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ddl string
		if err = rows.Scan(&ddl); err != nil {
			return nil, err
		}
		ddls = append(ddls, ddl+";")
	}

	return ddls, nil
}

func (d *DuckDBDatabase) indexes() ([]string, error) {
	var ddls []string
	const query = `
		SELECT sql
		FROM sqlite_master
		WHERE type = 'index' AND sql IS NOT NULL
		ORDER BY sql
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ddl string
		if err = rows.Scan(&ddl); err != nil {
			return nil, err
		}
		ddls = append(ddls, ddl+";")
	}

	return ddls, nil
}

func (d *DuckDBDatabase) DB() *sql.DB {
	return d.db
}

func (d *DuckDBDatabase) Close() error {
	return d.db.Close()
}

func (d *DuckDBDatabase) GetDefaultSchema() string {
	return ""
}

func (d *DuckDBDatabase) SetGeneratorConfig(config database.GeneratorConfig) {
	d.generatorConfig = config
}

func (d *DuckDBDatabase) GetGeneratorConfig() database.GeneratorConfig {
	return d.generatorConfig
}

func (d *DuckDBDatabase) GetTransactionQueries() database.TransactionQueries {
	return database.TransactionQueries{
		Begin:    "BEGIN",
		Commit:   "COMMIT",
		Rollback: "ROLLBACK",
	}
}

func (d *DuckDBDatabase) GetConfig() database.Config {
	return d.config
}
