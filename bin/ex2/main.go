package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func InsertAnn(db *sql.DB, field string) error {
	tx, err := db.Begin()

	if err != nil {
		return fmt.Errorf("InsertAnn: transaction begin: %w", err)
	}

	r, err := db.Exec(`INSERT INTO Table1(Field) VALUES (?)`, field)
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
		return fmt.Errorf("unexpected number of rows affected: %v", nr)
	}

	return nil
}

func GetAnn(db *sql.DB) (string, error) {
	r, err := db.Query(`
		SELECT		Field
		FROM		Table1
		;
	`)
	if err != nil {
		return "", fmt.Errorf("while query: %w", err)
	}
	for r.Next() {
		var ret string
		if err := r.Scan(&ret); err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("no rows for query: field=%v", "")
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
	db, err := sql.Open("sqlite3", "test.sqlite")
	if err != nil {
		fmt.Printf("open: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE
			Table1(
				Id		INTEGER PRIMARY KEY AUTOINCREMENT,
				Field	TEXT
			);
		`)
	if err != nil {
		fmt.Printf("Exec CREATE TABLE: %v", err)
	}

	if err := InsertAnn(db, "field"); err != nil {
		fmt.Printf("InsertAnn: %v", err)
	}

	// Error: GetAnn: Next error: bad parameter or other API misuse: not an error (21)
	// Why?  The query is literally `SELECT Field FROM Table1;`.
	s, err := GetAnn(db)
	if err != nil {
		fmt.Printf("GetAnn: %v", err)
	}
	fmt.Printf("got: %v", s)
}
