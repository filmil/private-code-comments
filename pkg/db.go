// Database operations
package pkg

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/golang/glog"
)

const (
	Pragmas = `?cache=shared&mode=memory`
	// For the time being, use an in-memory database.
	DefaultFilename = `file:test.db` + Pragmas
	DefaultSocket   = `:stdstream:`
)

const (
	// SqliteDriver is the name of the used SQL driver module.
	SqliteDriver = `sqlite3`

	// This is how to delete annotations.
	deleteDeltaStatementStr = `
		BEGIN TRANSACTION;

		DELETE FROM TABLE	AnnotationLocations
		WHERE				File = ? AND
							Line >= ? AND Line <= ?
		;

		UPDATE TABLE		AnnotationLocations
		SET					Line = Line + ?
		WHERE				File = ? AND Line > ?
		;

		COMMIT;`

	InsertDeltaStatementStr = `
		BEGIN TRANSACTION;

		UPDATE TABLE	AnnotationLocations
		SET				Line = Line + ?
		WHERE			File = ? AND Line > ?

		COMMIT;`
)

// CreateDBFile creates an empty database file at the given name.
// Returns true if the database needs to be initialized.
func CreateDBFile(dbFilename string) (bool, error) {
	var needsInit bool
	if dbFilename == DefaultFilename {
		needsInit = true
	} else {
		_, err := os.Stat(dbFilename)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return false, fmt.Errorf("unknown error: %v: %w", dbFilename, err)
			}
			// No such file, create it and set for schema creation.
			_, err := os.Create(dbFilename)
			if err != nil {
				return false, fmt.Errorf("could not create: %v: %w", dbFilename, err)
			}

			// Add the pragma suffixes
			if !strings.HasSuffix(dbFilename, Pragmas) {
				dbFilename = fmt.Sprintf("%s%s", dbFilename, Pragmas)
			}
			needsInit = true
		}
	}
	return needsInit, nil
}

func CreateDBSchema(db *sql.DB) error {
	if err := CreateSchema(db); err != nil {
		return fmt.Errorf("could not create: %v", err)
	}
	return nil
}

// CreateSchema creates the database with the appropriate file pkg.
func CreateSchema(db *sql.DB) error {
	const createStatementStr = `
		BEGIN TRANSACTION;

		-- Each annotation is in a separate table row.
		CREATE TABLE
			Annotations (
				Id		INTEGER PRIMARY KEY AUTOINCREMENT,
				Content TEXT NOT NULL
			);

		-- Each annotation location refers to an uniquely identified annotation.
		-- This allows us to change locations quickly.
		CREATE TABLE
			AnnotationLocations (
				Id			INTEGER PRIMARY KEY AUTOINCREMENT,
				Workspace	TEXT NOT NULL,
				Path		TEXT NOT NULL,
				Line		INTEGER,
				AnnId		INTEGER,

				FOREIGN KEY(AnnId) REFERENCES Annotations(Id)
					ON DELETE CASCADE
			);

		-- We will be querying by workspace and path often, so add the index.
		CREATE UNIQUE INDEX
			AnnotationsByFile
		ON
			AnnotationLocations(
				Workspace,
				Path,
				Line
			);

		COMMIT;`
	_, err := db.Exec(createStatementStr)
	if err != nil {
		return fmt.Errorf("could not create db: %w", err)
	}

	// TODO: fmil - Turn this off later.
	return InsertAnn(db, "", "/test.txt", 0, "This is a test entry.")
}

// InsertAnn inserts an annotation into the database.
// The annotation line must not previously exist.
//
// Args:
//   - workspace: the workspace, either an URI or a symbolic prefix.
//   - path: the file path relative to the workspace. For example,
//     for ws="file://dir", and file URI
//     "file://dir/file.txt", then path should be "/file.txt".
func InsertAnn(db *sql.DB, workspace, path string, line uint32, text string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("InsertAnn: transaction begin: %w", err)
	}

	r, err := db.Exec(
		`INSERT INTO Annotations(Content) VALUES (?);`, text)
	if err != nil {
		return fmt.Errorf("InsertAnn: exec1: %w", err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		return fmt.Errorf("InsertAnn: lastinsertid: %w", err)
	}

	const insertAnnLocStmtStr = `
		INSERT INTO AnnotationLocations(Workspace, Path, Line, AnnId)
			VALUES (?, ?, ?, ?)
		;`
	r, err = db.Exec(insertAnnLocStmtStr, workspace, path, line, id)
	if err != nil {
		return fmt.Errorf("InsertAnn: exec2: %w", err)
	}
	return tx.Commit()
}

// DeleteAnn deletes an annotation for the specific workspace, path and line.
// The annotation must exist
func DeleteAnn(db *sql.DB, workspace, path string, line uint32) error {
	r, err := db.Exec(`
		-- The Annotations table entry is deleted by cascade.
		DELETE FROM	AnnotationLocations
		WHERE		Workspace = ?
				AND
					Path = ?
				AND
					Line = ?
	;`, workspace, path, line)
	if err != nil {
		return fmt.Errorf("could not delete: workspace=%v, path=%v, line=%v", workspace, path, line)
	}
	ra, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: workspace=%v, path=%v, line=%v", workspace, path, line)
	}
	if ra != 1 {
		return fmt.Errorf("should affect exactly one row, but affected: %v", ra)
	}
	return nil
}

func MoveAnn(db *sql.DB, workspace, path string, line uint32, newPath string, newLine uint32) error {
	r, err := db.Exec(`
		UPDATE		AnnotationLocations
		SET			Path = ?, Line = ?
		WHERE		Workspace = ?
				AND
					Path = ?
				AND
					Line = ?
	;`, newPath, newLine, workspace, path, line)
	if err != nil {
		return fmt.Errorf("could not move: workspace=%v, path=%v, line=%v: %w", workspace, path, line, err)
	}
	ra, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: workspace=%v, path=%v, line=%v: %w", workspace, path, line, err)
	}
	if ra != 1 {
		return fmt.Errorf("should affect exactly one row, but affected: %v", ra)
	}
	return nil
}

// BulkMoveAnn moes annotation locations starting from given line by 'delta'.
//
// Note: firstLine is zero-indexed.
func BulkMoveAnn(db *sql.DB, workspace, path string, firstLine uint32, delta int32) error {
	_, err := db.Exec(`
		UPDATE			AnnotationLocations
		SET				Line = Line + ?
		WHERE			Workspace = ?
					AND
						Path = ?
					AND
						Line >= ?
		;`, delta, workspace, path, firstLine)
	if err != nil {
		return fmt.Errorf(
			"BulkMoveAnn: could not move annotations: ws=%q, file=%q, startLine=%v, delta=%v:\n\t%v",
			workspace, path, firstLine, delta, err)
	}
	return nil
}

// BulkDeleteAnn bulk-deletes annotations.
func BulkDeleteAnn(db *sql.DB, workspace, path string, firstLine uint32, lastLine uint32, delta int32) error {
	// Check invariants.
	if firstLine > lastLine {
		return fmt.Errorf("firstline: %v, lastline: %v: lastline must not be smaller", firstLine, lastLine)
	}
	l := int32(firstLine) - int32(lastLine)
	if delta < l {
		return fmt.Errorf("delta: %v, firstline: %v, lastline: %v, l: %v: diff must not be smaller",
			delta, firstLine, lastLine, l)
	}

	_, err := db.Exec(`
		BEGIN TRANSACTION;

		DELETE FROM		AnnotationLocations
		WHERE
						Workspace = ?
					AND
						Path = ?
					AND
						Line >= ?
					AND
						Line <= ?
		;

		UPDATE			AnnotationLocations
		SET				Line = Line + ?
		WHERE
						Workspace = ?
					AND
						Path = ?
					AND
						Line >= ?
		;

		COMMIT;
	;`, workspace, path, firstLine, lastLine, delta, workspace, path, lastLine+1)
	if err != nil {
		return fmt.Errorf(
			"BulkDeleteAnn: could not move annotations: ws=%q, file=%q, startLine=%v, lastLine=%v, delta=%v:\n\t%v",
			workspace, path, firstLine, lastLine, delta, err)
	}
	return nil
}

// GetAnn retrieves a single annotation.  Or an error if that particular annotation
// does not exist.
func GetAnn(db *sql.DB, workspace, path string, line uint32) (string, error) {
	const readAnnStmtStr = `
		SELECT		Content
		FROM		AnnotationLocations
		INNER JOIN	Annotations
		ON			AnnotationLocations.AnnId = Annotations.Id
		WHERE
			AnnotationLocations.Workspace = ?
				AND
			AnnotationLocations.Path = ?
				AND
			AnnotationLocations.Line = ?
		;`
	row := db.QueryRow(readAnnStmtStr, workspace, path, line)
	var ret string
	if err := row.Scan(&ret); err != nil {
		if err == sql.ErrNoRows {
			glog.Warningf("no rows for query: workspace=%v, path=%v, line=%v", workspace, path, line)
		} else {
			return "", fmt.Errorf("GetAnn: scan: %w, %q", err, ret)
		}
	}
	return ret, nil
}

// A single annotation
type Ann struct {
	Line    uint32
	Content string
}

// GetAnns returns all annotations for the given path in the workspace.
func GetAnns(db *sql.DB, workspace, path string) ([]Ann, error) {
	ret := []Ann{}
	r, err := db.Query(`
		SELECT		Line, Content
		FROM		AnnotationLocations
		INNER JOIN	Annotations
		ON			AnnotationLocations.AnnId = Annotations.Id
		WHERE
			AnnotationLocations.Workspace = ?
				AND
			AnnotationLocations.Path = ?
		ORDER BY	Line
	;`, workspace, path)

	for r.Next() {
		var ann Ann
		if err := r.Scan(&ann.Line, &ann.Content); err != nil {
			return nil, fmt.Errorf("could not scan: %w", err)
		}
		ret = append(ret, ann)
	}
	glog.V(2).Infof("GetAnns(ws=%q, file=%q): %+v", workspace, path, ret)

	return ret, err
}
