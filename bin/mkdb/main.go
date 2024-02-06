// A program that creates the pcc database and runs an arbitrary query on it.
package main

import (
	"flag"

	"github.com/filmil/private-code-comments/nvim_testing"
	"github.com/filmil/private-code-comments/pkg"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
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

	_, closeDB, err := nvim_testing.RunDBQuery(dbFilename, dbQueryFile)
	if err != nil {
		glog.Fatalf("could not run DB query: %v:", err)
	}
	defer closeDB()
}
