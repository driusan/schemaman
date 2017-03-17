package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/driusan/dgit/git"
)

type ColDefinition struct {
	Name       ColumnName
	Nullable   bool
	DataType   string
	ColDefault string
}

func ImportColumn(c *git.Client, tablename string, col ColDefinition) error {
	coldir := fmt.Sprintf("%s/.schema/tables/%s/%s/", c.WorkDir, tablename, col.Name)
	if err := os.MkdirAll(coldir, 0755); err != nil {
		return err
	}
	ioutil.WriteFile(coldir+"type", []byte(col.DataType), 0644)
	if !col.Nullable {
		ioutil.WriteFile(coldir+"not_null", []byte(""), 0644)
	}
	if col.ColDefault != "" {
		ioutil.WriteFile(coldir+"default", []byte(col.ColDefault), 0644)
	}
	return nil
}

func importTableMetadata(c *git.Client, tablename, primkey, colorder, engine string) error {
	metafile := fmt.Sprintf("%s/.schema/tables/%s/.metadata", c.WorkDir, tablename)
	var data string

	if primkey != "" {
		data += "Primary Key: " + primkey + "\n"
	}
	if engine != "" {
		data += "Engine: " + engine + "\n"
	}
	if colorder != "" {
		data += "Column Order: " + colorder + "\n"
	}

	if data != "" {
		return ioutil.WriteFile(metafile, []byte(data), 0644)
	}
	return nil
}

func Import(c *git.Client, args []string) error {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	host := flags.String("host", "", "Database host to import from")
	port := flags.Int("port", 3306, "Database port")
	username := flags.String("user", "", "Database username")

	// FIXME: This should have a way to read the password from STDIN.
	password := flags.String("password", "", "Database password")
	dbname := flags.String("dbname", "", "Database to import schema from")
	flags.Parse(args)
	if *host == "" || *dbname == "" || *username == "" {
		fmt.Fprintf(os.Stderr, "Usage:\n\tschemaman import [options]\n\nAvailable options:\n")
		flags.PrintDefaults()
		return fmt.Errorf("No host provided.")
	}
	con := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", *username, *password, *host, *port, *dbname)
	db, err := sql.Open("mysql", con)
	if err != nil {
		return err
	}

	// Get a list of every column to create the files in .schema on the filesystem
	columns, err := db.Prepare(
		`SELECT Table_name, Column_name, Column_type, Is_Nullable, Column_Default
		FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=?`)
	if err != nil {
		return err
	}

	rows, err := columns.Query(*dbname)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tblname, colname, datatype, nullable, coldefault string
		rows.Scan(&tblname, &colname, &datatype, &nullable, &coldefault)
		if err := ImportColumn(c, tblname, ColDefinition{
			Name:       ColumnName(colname),
			Nullable:   nullable == "YES",
			DataType:   datatype,
			ColDefault: coldefault,
		}); err != nil {
			return err
		}

	}

	// Populate the metadata for tables.
	// Note that while the "show" command supports CharSet, the information_schema
	// doesn't have any way for import to extract that information.
	// Column ordering for tables and db engine from information_schema.TABLES.
	tablemeta, err := db.Prepare(`SELECT t.Table_name,
		GROUP_CONCAT(c.Column_name ORDER BY c.ORDINAL_POSITION),
		t.Engine
	FROM information_schema.TABLES t
	JOIN information_schema.COLUMNS c
	ON(t.Table_name=c.Table_name AND t.TABLE_SCHEMA=c.TABLE_SCHEMA)
	WHERE t.TABLE_SCHEMA=?
	GROUP BY t.Table_name`)
	if err != nil {
		return err
	}

	// Primary keys are looked up in KEY_COLUMN_USAGE.
	tableKeys, err := db.Prepare(`SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY ORDINAL_POSITION)
	FROM information_schema.KEY_COLUMN_USAGE
	WHERE CONSTRAINT_SCHEMA=? AND CONSTRAINT_NAME='PRIMARY' AND TABLE_NAME=?
	GROUP BY TABLE_NAME, COLUMN_NAME`)
	if err != nil {
		return err
	}

	rows, err = tablemeta.Query(*dbname)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tblname, colorder, engine, primkey string
		rows.Scan(&tblname, &colorder, &engine)
		err := tableKeys.QueryRow(*dbname, tblname).Scan(&primkey)
		if err != nil {
			// If there was an error, we can't rely on the value.
			// (It was likely just "no rows in result set", so not
			// setting the primary key in the .metadata file is
			// what we want anyways.)
			primkey = ""
		}
		importTableMetadata(c, tblname, primkey, colorder, engine)
	}

	return nil
}
