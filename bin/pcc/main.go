package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"

	"github.com/filmil/private-code-comments/pkg"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
	"go.lsp.dev/jsonrpc2"
)

func main() {
	// Set up glogging
	defer func() {
		glog.Flush()
		glog.Exit()
	}()

	var (
		// The database filename.
		dbFilename string
		// The communication socket filename.
		socketFile string
	)

	// Set up flags

	flag.StringVar(&dbFilename,
		"db", pkg.DefaultFilename, "The file name for the private comments")
	flag.StringVar(&socketFile,
		"socket-file", pkg.DefaultSocket,
		"The socket to use for communication")
	flag.Parse()

	// Allow net.Listen to create the comms socket - remove it if it exists.
	if err := os.Remove(socketFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			glog.Fatalf("could not remove socket: %v", err)
		}
		// If the file does not exist, we're done here.
	}

	if dbFilename != pkg.DefaultFilename {
		if err := pkg.MakeAllDirs(path.Base(dbFilename)); err != nil {
			glog.Fatalf("could not make directories for: %v: %v", dbFilename, err)
		}
	} else {
		glog.V(3).Infof("not making a directory for: %v", dbFilename)
	}

	needsInit, err := pkg.CreateDBFile(dbFilename)
	if err != nil {
		glog.Fatalf("could not create db file: %v: %v", dbFilename, err)
	}

	// connect and schedule cleanup
	db, err := sql.Open(pkg.SqliteDriver, dbFilename)
	if err != nil {
		glog.Fatalf("could not open database: %v: %v", dbFilename, err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			glog.Infof("error closing database: %v: %v", dbFilename, err)
		}
	}()

	// Create the data schema if it has not been created before.
	glog.Infof("creating a new database: %s", dbFilename)
	if needsInit {
		if err := pkg.CreateDBSchema(db); err != nil {
			glog.Fatalf("could not create: %v: %v", dbFilename, err)
		}
	}

	// Some dummy operations for the time being.

	// get SQLite version
	r := db.QueryRow("select sqlite_version()")
	var dbVer string
	if err := r.Scan(&dbVer); err != nil {
		glog.Fatalf("could not read db version: %v: %v", dbFilename, err)
	}
	glog.Infof("sqlite3 version: %v: %v", dbFilename, dbVer)

	var id jsonrpc2.ID

	glog.Infof("JSON-RPC2 id: %v", id)

	Serve(socketFile, db)

	glog.Infof("exiting program")
}

type StdioConn struct{}

// Close implements io.ReadWriteCloser.
func (*StdioConn) Close() error {
	return os.Stdout.Close()
}

// Read implements io.ReadWriteCloser.
func (*StdioConn) Read(p []byte) (n int, err error) {
	return os.Stdin.Read(p)
}

// Write implements io.ReadWriteCloser.
func (*StdioConn) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

var _ io.ReadWriteCloser = (*StdioConn)(nil)

func Serve(f string, db *sql.DB) error {
	glog.Infof("listening for a connection at: %v", f)

	if f == pkg.DefaultSocket {
		// Use a ReadWriteCloser from stdio and stout.
		jc := jsonrpc2.NewConn(jsonrpc2.NewStream(&StdioConn{}))
		ctx := context.Background()
		s, err := pkg.NewServer(ctx, db, jc)
		if err != nil {
			return fmt.Errorf("could not create server: %w", err)
		}
		srv := jsonrpc2.HandlerServer(s.GetHandlerFunc())
		if err := srv.ServeStream(ctx, jc); err != nil {
			if !errors.Is(err, pkg.ExitError) {
				glog.Infof("error while serving request: %v", err)
				return err
			}
		}
	} else {
		l, err := net.Listen("unix", f)
		if err != nil {
			return fmt.Errorf("could not listen to socket: %v: %v", f, err)
		}
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				return fmt.Errorf("could not accept a connection: %v", err)
			}

			// Create a json connection
			jc := jsonrpc2.NewConn(jsonrpc2.NewStream(c))

			ctx := context.Background()

			s, err := pkg.NewServer(ctx, db, jc)
			if err != nil {
				return fmt.Errorf("could not create server: %w", err)
			}
			srv := jsonrpc2.HandlerServer(s.GetHandlerFunc())
			// Probably needs to be in a separate goroutine, this.
			if err := srv.ServeStream(ctx, jc); err != nil {
				glog.Infof("error while serving request: %v", err)
				if errors.Is(err, pkg.ExitError) {
					break
				}
			}
		}
	}
	return nil
}
