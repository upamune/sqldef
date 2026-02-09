package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sqldef/sqldef/v3/database"
	"github.com/sqldef/sqldef/v3/database/duckdb"
	"github.com/sqldef/sqldef/v3/parser"
	"github.com/sqldef/sqldef/v3/schema"
	tu "github.com/sqldef/sqldef/v3/testutil"
)

const (
	applyPrefix     = "-- Apply --\n"
	nothingModified = "-- Nothing is modified --\n"
)

func wrapWithTransaction(ddls string) string {
	return applyPrefix + "BEGIN;\n" + ddls + "COMMIT;\n"
}

func TestApply(t *testing.T) {
	tests, err := tu.ReadTests("tests.yml")
	if err != nil {
		t.Fatal(err)
	}

	sqlParser := database.NewParser(parser.ParserModeSQLite3)
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dbFile := filepath.Join(t.TempDir(), "test.db")
			db, err := connectDatabaseByName(dbFile)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			tu.RunTest(t, db, test, schema.GeneratorModeSQLite3, sqlParser, "", "")
		})
	}
}

func TestDuckDBdefApplyAndIdempotency(t *testing.T) {
	resetTestDatabase()

	createTable := tu.StripHeredoc(`
		CREATE TABLE bigdata (
		  data integer
		);
	`)

	assertApplyOutput(t, createTable, wrapWithTransaction(createTable))
	assertApplyOutput(t, createTable, nothingModified)
}

func TestDuckDBdefDryRun(t *testing.T) {
	resetTestDatabase()
	tu.WriteFile("schema.sql", tu.StripHeredoc(`
		CREATE TABLE users (
			id integer NOT NULL PRIMARY KEY,
			age integer
		);
	`))

	dryRun := tu.MustExecute(t, "./duckdbdef", testDBName, "--dry-run", "--file", "schema.sql")
	apply := tu.MustExecute(t, "./duckdbdef", testDBName, "--file", "schema.sql")
	assert.Equal(t, strings.Replace(apply, "Apply", "dry run", 1), dryRun)
}

func TestDuckDBdefExport(t *testing.T) {
	resetTestDatabase()
	out := tu.MustExecute(t, "./duckdbdef", testDBName, "--export")
	assert.Equal(t, "-- No table exists --\n", out)

	mustDuckDBExec(testDBName, tu.StripHeredoc(`
		CREATE TABLE users (
			id integer NOT NULL PRIMARY KEY,
			age integer
		);
	`))

	out = tu.MustExecute(t, "./duckdbdef", testDBName, "--export")
	assert.Equal(t, tu.StripHeredoc(`
		CREATE TABLE users(id INTEGER PRIMARY KEY, age INTEGER);
	`), out)
}

func TestDuckDBdefHelp(t *testing.T) {
	_, err := tu.Execute("./duckdbdef", "--help")
	if err != nil {
		t.Errorf("failed to run --help: %s", err)
	}

	out, err := tu.Execute("./duckdbdef")
	if err == nil {
		t.Errorf("no database must be error, but successfully got: %s", out)
	}
}

const testDBName = "duckdbdef_test.db"

func TestMain(m *testing.M) {
	resetTestDatabase()
	tu.BuildForTest()
	status := m.Run()
	_ = os.Remove("duckdbdef")
	_ = os.Remove(testDBName)
	_ = os.Remove("schema.sql")
	os.Exit(status)
}

func assertApplyOutput(t *testing.T, desiredSchema string, expected string) {
	t.Helper()
	actual := assertApplyOutputWithConfig(t, desiredSchema, database.GeneratorConfig{EnableDrop: false, LegacyIgnoreQuotes: true})
	assert.Equal(t, expected, actual)
}

func assertApplyOutputWithConfig(t *testing.T, desiredSchema string, config database.GeneratorConfig) string {
	t.Helper()

	db, err := connectDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	sqlParser := database.NewParser(parser.ParserModeSQLite3)
	output, err := tu.ApplyWithOutput(db, schema.GeneratorModeSQLite3, sqlParser, desiredSchema, config)
	if err != nil {
		t.Fatal(err)
	}

	return output
}

func resetTestDatabase() {
	if _, err := os.Stat(testDBName); err == nil {
		err := os.Remove(testDBName)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func connectDatabase() (database.Database, error) {
	return duckdb.NewDatabase(database.Config{DbName: testDBName})
}

func connectDatabaseByName(dbName string) (database.Database, error) {
	return duckdb.NewDatabase(database.Config{DbName: dbName})
}

func duckdbExec(dbName string, statement string) error {
	db, err := duckdb.NewDatabase(database.Config{DbName: dbName})
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.DB().Exec(statement)
	return err
}

func mustDuckDBExec(dbName string, statement string) {
	if err := duckdbExec(dbName, statement); err != nil {
		panic(err)
	}
}
