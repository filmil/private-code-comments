package main

import (
	"database/sql"
	"fmt"

	_ "github.com/glebarez/go-sqlite"
)

func InsertAnn(db *sql.DB, workspace string) error {
	tx, err := db.Begin()

	if err != nil {
		return fmt.Errorf("InsertAnn: transaction begin: %w", err)
	}

	r, err := db.Exec(`INSERT INTO Table1(Workspace) VALUES (?)`, workspace)
	if err != nil {
		return fmt.Errorf("InsertAnn: exec2: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error on commit; %w", err)
	}

	nr, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("RowsAffected: %w", err)
	}
	if nr != 1 {
		return fmt.Errorf("Rows affected: %v", nr)
	}

	return nil
}

func GetAnn(db *sql.DB, _ string) (string, error) {
	const readAnnStmtStr = `
		SELECT		Workspace
		FROM		Table1
		;
	`
	r, err := db.Query(readAnnStmtStr)
	if err != nil {
		return "", fmt.Errorf("while query: %w", err)
	}
	for r.Next() {
		var ret string
		if err := r.Scan(&ret); err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("no rows for query: workspace=%v", "")
			} else {
				return "", fmt.Errorf("GetAnn: scan: %w, %q", err, ret)
			}
		}
		return ret, nil
	}
	if r.Err() != nil {
		return "", fmt.Errorf("Next error: %w", r.Err())
	}
	// Get here only if no hits.
	return "", fmt.Errorf("nothing found (???)")
}

func main() {
	db, err := sql.Open("sqlite", "test.sqlite?_pragma=foreign_keys(1)")
	if err != nil {
		fmt.Printf("open: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE
			Table1(
				Id			INTEGER PRIMARY KEY AUTOINCREMENT,
				Workspace	TEXT
			);
		`)
	if err != nil {
		fmt.Printf("exec create: %v", err)
	}

	if err := InsertAnn(db, "workspace"); err != nil {
		fmt.Printf("insert: %v", err)
	}

	_, err = GetAnn(db, "workspace")
	if err != nil {
		fmt.Printf("GetAnn: %v", err)
	}

}
