package nvim_testing

import (
	"database/sql"
	"fmt"

	"github.com/filmil/private-code-comments/pkg"
)

// RunDBQuery creates a database and runs a SQL query on it to populate it.
func RunDBQuery(dbFilename, query string) (*sql.DB, func(), error) {
	if dbFilename == "" {
		dbFilename = pkg.DefaultFilename
	}
	_, err := pkg.CreateDBFile(dbFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create db file: %v: %v", dbFilename, err)
	}

	db, err := sql.Open(pkg.SqliteDriver, dbFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open database: %v: %v", dbFilename, err)
	}
	if err := pkg.CreateDBSchema(db); err != nil {
		return nil, nil, fmt.Errorf("could not create: %v: %v", dbFilename, err)
	}

	if query != "" {
		_, err = db.Exec(query)
		if err != nil {
			return nil, nil, fmt.Errorf("could not execute query: %v", err)
		}
	}

	return db, func() { db.Close() }, nil
}
