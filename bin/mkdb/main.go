// A program that creates the pcc database and runs an arbitrary query on it.
package main

import (
	"database/sql"
	"flag"

	"github.com/filmil/private-code-comments/pkg"
	"github.com/golang/glog"
)

func main() {
	defer func() {
		glog.Flush()
	}()
	var (
		// The database name.
		dbFilename string
		// The query to execute on the database once it is created.
		dbQueryFile string
	)

	flag.StringVar(&dbFilename, "db", pkg.DefaultFilename, "The file name for the private comments database")
	flag.StringVar(&dbQueryFile, "query-file", "", "The query to execute on the created database.")

	flag.Parse()

	if dbQueryFile == "" {
		glog.Fatalf("flag --db=... is required")
	}

	_, err := pkg.CreateDBFile(dbFilename)
	if err != nil {
		glog.Fatalf("could not create db file: %v: %v", dbFilename, err)
	}

	db, err := sql.Open(pkg.SqliteDriver, dbFilename)
	if err != nil {
		glog.Fatalf("could not open database: %v: %v", dbFilename, err)
	}
	if err := pkg.CreateDBSchema(db); err != nil {
		glog.Fatalf("could not create: %v: %v", dbFilename, err)
	}

	_, err = db.Exec(dbQueryFile)
	if err != nil {
		glog.Fatalf("could not execute query: %v", err)
	}
}
