package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"strings"

	_ "github.com/glebarez/go-sqlite"
	"go.lsp.dev/jsonrpc2"
)

var (
	// createStatementStr is used to create a new table from scratch.
	createStatementStr = `
		CREATE TABLE
			Annotations (
				Id		INTEGER PRIMARY KEY AUTOINCREMENT,
				Content TEXT NOT NULL
			);

		CREATE TABLE
			AnnotationLocations (
				Id			INTEGER PRIMARY KEY AUTOINCREMENT,
				Workspace	TEXT NOT NULL,
				Path		TEXT NOT NULL,
				Line		INTEGER,
				FOREIGN KEY(Line) REFERENCES Anontations(Id)
			);

		CREATE UNIQUE INDEX
			AnnotationsByFile
		ON
			AnnotationLocations(
				Workspace,
				Path
			);
	`
)

type ID = int

// 1-based line index.
type Line = int

// Annotation represents a single annotation.
type Annotation struct {
	ID      ID
	Content string
}

// AnnotationLocation is a location
type AnnotationLocation struct {
	ID           ID
	Workspace    string
	Path         string
	Line         Line
	AnnotationID ID
}

const (
	pragmas = `?_pragma=foreign_keys(1)`
	// For the time being, use an in-memory database.
	defaultFilename = `:memory:` + pragmas
	defaultSocket   = `pcc.sock`
)

func CreateSchema(db *sql.DB) error {
	_, err := db.Exec(createStatementStr)
	if err != nil {
		return fmt.Errorf("could not create db: %v", err)
	}
	return nil
}

func main() {
	// Set up logging
	log.SetPrefix(fmt.Sprintf("%s: ", path.Base(os.Args[0])))
	log.SetFlags(log.Ldate | log.Lshortfile | log.Lmicroseconds | log.Lmsgprefix)

	var (
		// The database filename.
		dbFilename string
		// The communication socket filename.
		socketFile string
	)

	// Set up flags
	flag.StringVar(&dbFilename,
		"db", defaultFilename, "The file name for the private comments")
	flag.StringVar(&socketFile,
		"socket-file", path.Join(os.Getenv("XDG_RUNTIME_DIR"), defaultSocket),
		"The socket to use for communication")
	flag.Parse()

	// Allow net.Listen to create the comms socket - remove it if it exists.
	if err := os.Remove(socketFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Fatalf("could not remove socket: %v", err)
		}
		// If the file does not exist, we're done here.
	}

	var needsInit bool
	if dbFilename == defaultFilename {
		needsInit = true
	} else {
		_, err := os.Stat(dbFilename)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				log.Fatalf("unknown error: %v: %v", dbFilename, err)
			}
			// No such file, create it and set for schema creation.
			_, err := os.Create(dbFilename)
			if err != nil {
				log.Fatal(err)
			}

			// Add the pragma suffixes
			if !strings.HasSuffix(dbFilename, pragmas) {
				dbFilename = fmt.Sprintf("%s%s", dbFilename, pragmas)
			}
			needsInit = true
		}
	}

	// connect and schedule cleanup
	db, err := sql.Open("sqlite", dbFilename)
	if err != nil {
		log.Fatalf("could not open database: %v: %v", dbFilename, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing database: %v: %v", dbFilename, err)
		}
	}()

	// Create the data schema if it has not been created before.
	if needsInit {
		log.Printf("creating a new database: %s", dbFilename)
		if err := CreateSchema(db); err != nil {
			log.Fatalf("could not create: %v: %v", dbFilename, err)
		}
	}

	// Some dummy operations for the time being.

	// get SQLite version
	r := db.QueryRow("select sqlite_version()")
	var dbVer string
	if err := r.Scan(&dbVer); err != nil {
		log.Fatalf("could not read db version: %v: %v", dbFilename, err)
	}
	log.Printf("sqlite3 version: %v: %v", dbFilename, dbVer)

	var id jsonrpc2.ID

	log.Printf("JSON-RPC2 id: %v", id)

	Serve(socketFile, db)
}

func Serve(f string, db *sql.DB) error {
	log.Printf("listening for a connection at: %v", f)
	l, err := net.Listen("unix", f)
	if err != nil {
		return fmt.Errorf("could not listen to socket: %v: %v", f, err)
	}
	srv := jsonrpc2.HandlerServer(Handler)
	defer l.Close()
	for {
		c, err := l.Accept()
		if err != nil {
			return fmt.Errorf("could not accept a connection: %v", err)
		}

		// Create a json connection
		jc := jsonrpc2.NewConn(jsonrpc2.NewStream(c))

		ctx := context.Background()

		if err := srv.ServeStream(ctx, jc); err != nil {
			log.Printf("error while serving request: %v", err)
			// Don't exit.
		}
	}
	// Unreachable.
}

func Handler(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	log.Printf("request: %v", req)
	var d struct {
		A int `json:"a"`
		B int `json:"b"`
		C int `json:"c"`
	}
	if err := json.Unmarshal(req.Params(), &d); err != nil {
		return fmt.Errorf("could not parse request: %v: %v", req, err)
	}
	reply(ctx, nil, fmt.Errorf("TBD"))
	log.Printf("request params were: %+v", d)
	return fmt.Errorf("TBD")
}
