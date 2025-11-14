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
		// Tries to flush any logging buffers before exiting. This allows us
		// to capture more log output than without it.
		glog.Flush()
		glog.Exit()
	}()

	var (
		// The database filename.
		dbFilename string
		// The communication socket filename.
		socketFile string
		version    bool
	)

	// Set up flags
	flag.StringVar(&dbFilename,
		"db", pkg.DefaultFilename, "The file name for the private comments")
	flag.StringVar(&socketFile,
		"socket-file", pkg.DefaultSocket,
		"The socket to use for communication")
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.Parse()

	if version {
		fmt.Printf("%v\n", getVersion())
		os.Exit(0)
	}

	// Allow net.Listen to create the comms socket - remove it if it exists.
	if err := os.Remove(socketFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			glog.Fatalf("could not remove socket: %v", err)
		}
		// If the file does not exist, we're done here.
	}

	if dbFilename != pkg.DefaultFilename {
		if err := pkg.MakeAllDirs(path.Dir(dbFilename)); err != nil {
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

	if err := Serve(socketFile, db); err != nil {
		glog.Errorf("error while serving: %v", err)
	}
	glog.Infof("exiting program")
}

// StdioConn is a connection that uses stdin for input, and stdout for output.
type StdioConn struct{}

var _ io.ReadWriteCloser = (*StdioConn)(nil)

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

// Serve serves LSP on socketName, using the database `db`.
//
// If socketName is the special constant `pkg.DefaultSocket`, then LSP is
// served on a socket created by joining stdin/stdout, as LSP servers usually
// do.
func Serve(socketName string, db *sql.DB) error {
	glog.Infof("listening for a connection at: %v", socketName)

	if socketName == pkg.DefaultSocket {
		// Use a ReadWriteCloser from stdin and stdout.
		stream := jsonrpc2.NewStream(&StdioConn{})
		if err := ServeSingleConn(db, stream); err != nil {
			glog.Infof("error while serving a signle request: %v", err)
		}
	} else {
		l, err := net.Listen("unix", socketName)
		if err != nil {
			return fmt.Errorf("could not listen to socket: %v: %v", socketName, err)
		}
		defer l.Close()
		for {
			c, err := l.Accept()
			if err != nil {
				return fmt.Errorf("could not accept a connection: %v", err)
			}

			// Create a json connection
			stream := jsonrpc2.NewStream(c)
			if err := ServeSingleConn(db, stream); err != nil {
				if !errors.Is(err, pkg.ExitError) {
					glog.Infof("error: %v", err)
				} else {
					// With pkg.ExitError, we exit the loop and are done.
					break
				}
			}
		}
	}
	return nil
}

// ServeSingleConn serves a single JSON-RPC2 connection on `stream`, using `db`
// as the source of annotations.
//
// A special error `pkg.ExitError` means that an exit is requested.  `nil` means
// no error, and the caller may try to repeat serving.
func ServeSingleConn(db *sql.DB, stream jsonrpc2.Stream) error {
	jc := jsonrpc2.NewConn(stream)
	ctx := context.Background()
	s, err := pkg.NewServer(ctx, db, jc)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}
	srv := jsonrpc2.HandlerServer(s.GetHandlerFunc())
	if err := srv.ServeStream(ctx, jc); err != nil {
		glog.Infof("error while serving request: %v", err)
		if !errors.Is(err, pkg.ExitError) {
			return fmt.Errorf("error while serving request: %w", err)
		}
		return err
	}
	return nil
}
