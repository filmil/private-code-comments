// Database operations
package pkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/golang/glog"
)

const (
	// Pragmas are used to initialize an in-memory database. Useful for tests.
	Pragmas = `?cache=shared&mode=memory`
	// For the time being, use an in-memory database.
	DefaultFilename = `file:test.db` + Pragmas
	// This socket name causes using stdin/stdout instead of a specific unix
	// domain socket.
	DefaultSocket = `:stdstream:`
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
//
// Returns true if the database needs to be initialized, e.g. if an empty
// file without a schema was created.
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

// CreateDBSchema creates the data schema used in this program in an empty
// database db.
func CreateDBSchema(db *sql.DB) error {
	if err := CreateSchema(db); err != nil {
		return fmt.Errorf("could not create: %w", err)
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
		return fmt.Errorf("could not create: %w", err)
	}
	return nil
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
	glog.V(2).Infof("db/InsertAnn: ws=%v, path=%v, line=%v", workspace, path, line)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	r, err := db.Exec(`INSERT INTO Annotations(Content) VALUES (?);`, text)
	if err != nil {
		return fmt.Errorf("could not exec: %w", err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last insert ID: %w", err)
	}
	const insertAnnLocStmtStr = `
		INSERT INTO AnnotationLocations(Workspace, Path, Line, AnnId) VALUES (?, ?, ?, ?)
        ON CONFLICT(Workspace, Path, Line)
        DO UPDATE SET AnnId=?
		;`
	r, err = db.Exec(insertAnnLocStmtStr, workspace, path, line, id, id)
	if err != nil {
		return fmt.Errorf("could not exec statement: %w", err)
	}
	return tx.Commit()
}

// DeleteAnn deletes an annotation for the specific workspace, path and line.
// The annotation does not need to exist.
func DeleteAnn(db *sql.DB, workspace, path string, line uint32) error {
	glog.V(2).Infof("db/DeleteAnn: ws=%v, path=%v, line=%v", workspace, path, line)
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
	// It is allowed for an annotation *not* to exist when requested a delete.
	if ra > 1 {
		return fmt.Errorf("should affect exactly one row, but affected: %v", ra)
	}
	return nil
}

// MoveAnn moves a single annotation from a file location to another location in a possibly different file.
func MoveAnn(db *sql.DB, workspace, path string, line uint32, newPath string, newLine uint32) error {
	glog.V(2).Info("db/MoveAnn: ws=%v, path=%v, line=%v -> newPath=%v, newLine=%v",
		workspace, path, line, newPath, newLine)
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
		return fmt.Errorf("could not move: workspace=%v, path=%v, line=%v: %w",
			workspace, path, line, err)
	}
	ra, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: workspace=%v, path=%v, line=%v: %w",
			workspace, path, line, err)
	}
	if ra != 1 {
		return fmt.Errorf("should affect exactly one row, but affected: %v", ra)
	}
	return nil
}

// BulkMoveAnn moves annotation locations starting from given line to EOF by 'delta'.
//
// Note: firstLine is zero-indexed.
func BulkMoveAnn(db *sql.DB, workspace, path string, firstLine uint32, delta int32) error {
	tx, err := db.BeginTx(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("could not create TX: %v", err)
	}
	err = TxBulkMoveAnn(tx, workspace, path, firstLine, delta)
	if err != nil {
		return fmt.Errorf("could not schedule TX: %v", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"BulkMoveAnn: could not move annotations: ws=%q, file=%q, startLine=%v, delta=%v:\n\t%v",
			workspace, path, firstLine, delta, err)
	}
	return nil
}

// TxBulkMoveAnn schedules a BulkMoveAnn into a transaction.
func TxBulkMoveAnn(tx *sql.Tx, workspace, path string, firstLine uint32, delta int32) error {
	glog.V(2).Infof("db/TxBulkMoveAnn: ws=%q, path=%q, firstLine=%v, delta=%v",
		workspace, path, firstLine, delta)
	_, err := tx.Exec(`
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
			"BulkMoveAnn: could not move annotations: ws=%q, file=%q, startLine=%v, delta=%v:\n\t%w",
			workspace, path, firstLine, delta, err)
	}
	return nil
}

func addConcat(tx *sql.Tx, workspace, path string, firstline, lastline uint32) (sql.Result, error) {
	r, err := tx.Exec(`
        -- Insert the concatenation of content of all affected lines to
        -- the first line.
        -- Save the generated ID into r above.
        INSERT OR IGNORE INTO Annotations(Content)
            SELECT group_concat(Content, ?) -- check how to use a separator
            FROM    AnnotationLocations
            INNER JOIN  Annotations
            ON  AnnotationLocations.AnnId = Annotations.ID
            WHERE AnnotationLocations.Workspace = ?     -- workspace
                    AND
                  AnnotationLocations.Path = ?          -- path
                    AND
                  AnnotationLocations.Line >= ?         -- firstline
                    AND
                  AnnotationLocations.Line <= ?          -- lastline
            ORDER BY AnnotationLocations.Line
        ;`, "\n--\n", workspace, path, firstline, lastline)
	if err != nil {
		return r, fmt.Errorf("could not add concat: %w", err)
	}
	return r, nil
}

// TxBulkAppendAnn schedules an append in order of all the annotations on the file path between firstline
// and lastline in the appropriate sequence.
func TxBulkAppendAnn(tx *sql.Tx, workspace, path string, firstline, lastline uint32, delta int32) error {
	r, err := addConcat(tx, workspace, path, firstline, lastline)
	if err != nil {
		return fmt.Errorf("could not bulk append: %w", err)
	}
	annID, err := r.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last insert ID: %w", err)
	}

	_, err = tx.Exec(`
        -- delete the notes from the deleted section.
        -- These are already replaced by the concatenation above.
        -- The annotation contents are deleted through cascade.
        DELETE FROM AnnotationLocations
        WHERE AnnotationLocations.Workspace = ? -- workspace
                AND
              AnnotationLocations.Path = ?
                AND
              AnnotationLocations.Line >= ? -- firstline
                AND
              AnnotationLocations.Line <= ? -- lastline
        ;`, workspace, path, firstline, lastline)

	if err != nil {
		return fmt.Errorf("could not schedule exec")
	}

	_, err = tx.Exec(`
        INSERT OR REPLACE INTO AnnotationLocations(Workspace, Path, Line, AnnId)
        VALUES (?, ?, ?, ?)
        ;`, workspace, path, firstline, annID)
	if err != nil {
		return fmt.Errorf("could not insert")
	}

	if err := TxBulkMoveAnn(tx, workspace, path, lastline, delta); err != nil {
		return fmt.Errorf("could not commit")
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
	if workspace == "" || path == "" {
		return "", fmt.Errorf("GetAnn: empty workspace or path: ws=%q, path=%q", workspace, path)
	}
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
	// Line is the 0-based line index of the line where the annotation is.
	Line uint32
	// Content is the string content of the annotation.
	Content string
}

// GetRawAnns gets all the annotations from the database.
func GetRawAnns(db *sql.DB) ([]Ann, error) {
	ret := []Ann{}
	r, err := db.Query(`
		SELECT		Id, Content
		FROM		Annotations
		ORDER BY	Id
	;`)
	if err != nil {
		return nil, fmt.Errorf("could not query: %w", err)
	}

	for r.Next() {
		var ann Ann
		if err := r.Scan(&ann.Line, &ann.Content); err != nil {
			return nil, fmt.Errorf("could not scan: %w", err)
		}
		ret = append(ret, ann)
	}
	glog.V(2).Infof("GetRawAnns: %+v", ret)
	return ret, err
}

// GetAnns returns all annotations for the given path in the workspace.
func GetAnns(db *sql.DB, workspace, path string) ([]Ann, error) {
	if workspace == "" || path == "" {
		return nil, fmt.Errorf("GetAnns: empty workspace or path: ws=%q, path=%q", workspace, path)
	}
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
	if err != nil {
		return nil, fmt.Errorf("GetAnns: query failed: %v", err)
	}

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
