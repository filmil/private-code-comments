package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/filmil/pcc/pkg"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
)

const (
	pragmas = `?cache=shared&mode=memory`
	// For the time being, use an in-memory database.
	defaultFilename = `file:test.db` + pragmas
	defaultSocket   = `:stdstream:`
)

func main() {
	// Set up glogging
	defer func() {
		glog.Flush()
	}()

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
		"socket-file", defaultSocket,
		"The socket to use for communication")
	flag.Parse()

	// Allow net.Listen to create the comms socket - remove it if it exists.
	if err := os.Remove(socketFile); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			glog.Fatalf("could not remove socket: %v", err)
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
				glog.Fatalf("unknown error: %v: %v", dbFilename, err)
			}
			// No such file, create it and set for schema creation.
			_, err := os.Create(dbFilename)
			if err != nil {
				glog.Fatal(err)
			}

			// Add the pragma suffixes
			if !strings.HasSuffix(dbFilename, pragmas) {
				dbFilename = fmt.Sprintf("%s%s", dbFilename, pragmas)
			}
			needsInit = true
		}
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
	if needsInit {
		glog.Infof("creating a new database: %s", dbFilename)
		if err := pkg.CreateSchema(db); err != nil {
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

type Server struct {
	// For sending notifications.
	conn jsonrpc2.Conn

	// Server init state.
	gotInitialize  bool
	gotInitialized bool
	gotShutdown    bool

	// Info from the `initialize` call.
	clientInfo       *lsp.ClientInfo
	workspaceFolders []lsp.WorkspaceFolder

	// Closed when the initialized message is sent.
	initialized     chan struct{}
	diagnosticQueue chan lsp.URI
	globalCtx       context.Context
	db              *sql.DB
	cancel          context.CancelFunc

	// Just a temporary thing.
	count int
}

// Finds the workspace that the file with uri URI belongs to.
// Returns the workspace URI encoded as string, and the relative
// path for the provided file.
func (s *Server) FindWorkspace(uri lsp.URI) (string, string) {
	u := string(uri)
	for _, ws := range s.workspaceFolders {
		if strings.HasPrefix(u, ws.URI) {
			r := RPath(ws.URI, uri)
			return ws.URI, r
		}
	}
	return "", u
}

func NewServer(ctx context.Context, db *sql.DB, conn jsonrpc2.Conn) (*Server, error) {
	// Initialize the database.
	ctx, cancel := context.WithCancel(ctx)

	s := Server{
		diagnosticQueue: make(chan lsp.URI, 1),
		initialized:     make(chan struct{}, 1),
		globalCtx:       ctx,
		db:              db,
		cancel:          cancel,
		conn:            conn,
	}

	go s.DiagnosticsFn()

	return &s, nil
}

type LineRange struct {
	// Start is zero based.
	Start uint32
	// End is zero based.
	End uint32
}

func NewLineRange(r lsp.Range) LineRange {
	return LineRange{
		Start: r.Start.Line,
		End:   r.End.Line,
	}
}

// NumLines returns the number of lines spanned by this line range.
func (l LineRange) NumLines() uint32 {
	return l.End - l.Start
}

// MakeDiagnostic creates a single diagnostic line.
func MakeDiagnostic(lr LineRange, m string) lsp.Diagnostic {
	ret := lsp.Diagnostic{
		Range: lsp.Range{
			Start: lsp.Position{
				Line: lr.Start,
			},
			End: lsp.Position{
				Line: lr.End,
			},
		},
		Severity: lsp.DiagnosticSeverityHint,
		Source:   "private comments",
		Message:  m,
	}
	return ret
}

func (s *Server) Shutdown() {
	s.clientInfo = nil
	s.gotInitialized = false
	s.gotInitialize = false
	s.gotShutdown = true
}

// RPath returns a file path relative to the given workspace.
func RPath(ws string, uri lsp.URI) string {
	f := string(uri)
	if !strings.HasPrefix(f, ws) {
		panic(fmt.Sprintf("ws is not a prefix: ws=%q, file=%v", ws, uri))
	}
	return strings.TrimPrefix(ws, string(uri))
}

func (s *Server) DiagnosticsFn() {
	<-s.initialized
	glog.V(1).Infof("diagnostics up and running")
	ctx, cancel := context.WithCancel(s.globalCtx)
	defer cancel()
	if err := s.conn.Notify(ctx, "$/moops", "oops"); err != nil {
		glog.Errorf("oops: %v", err)
	}
	for {
		glog.Flush()
		select {
		case <-s.globalCtx.Done():
			break
		case uri := <-s.diagnosticQueue:
			glog.V(1).Infof("queue tick.")
			ws, rpath := s.FindWorkspace(uri)
			anns, err := pkg.GetAnns(s.db, "" /* ws */, rpath)
			if err != nil {
				glog.Errorf("error getting annotations: workspace=%v, file=%v: %v", ws, rpath, err)
			}
			if len(anns) == 0 {
				glog.V(1).Infof("DiagnosticsFn: nothing to publish.")
				return
			}
			var d []lsp.Diagnostic
			for _, a := range anns {
				d = append(d, MakeDiagnostic(
					LineRange{Start: a.Line, End: a.Line + 1}, a.Content))
			}
			p := lsp.PublishDiagnosticsParams{URI: uri, Diagnostics: d}
			if err := s.conn.Notify(
				ctx, lsp.MethodTextDocumentPublishDiagnostics, &p); err != nil {
				glog.Errorf("DiagnosticsFn: error while publishing diagnostics for: %v", uri)
			}
		}
	}
}

func (s *Server) MoveAnnotations(ctx context.Context, lr LineRange, delta int32, uri lsp.URI) error {
	if delta == 0 {
		// We already excluded delta==0 when calling here.
		glog.Fatalf("delta==0: this should not happen")
	}

	if delta > 0 {
		// When lines are inserted, we increment the line number of all annotations from the
		// line after the insert line by the number of inserted lines.
		ws, rpath := s.FindWorkspace(uri)
		if err := pkg.BulkMoveAnn(s.db, ws, rpath, lr.Start, delta); err != nil {
			return fmt.Errorf("MoveAnnotations: %w", err)
		}
	}
	if delta < 0 {
		glog.Errorf("delta<0: TBD")
	}

	// Refresh diagnostics
	s.diagnosticQueue <- uri
	return nil
}

// GetHandlerFunc returns a stateful function that can be given to jsonrpc2.StreamServer
// to serve JSON-RPC2 requests.
func (s *Server) GetHandlerFunc() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		glog.Infof("JSON-RPC2 Request method: %v", req.Method())
		defer func() {
			glog.Flush()
		}()

		switch req.Method() {
		case lsp.MethodTextDocumentDidSave:
			var p lsp.DidSaveTextDocumentParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didSave: %v", err)
			}
		case lsp.MethodTextDocumentDidOpen:
			var p lsp.DidOpenTextDocumentParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didOpen: %v", err)
			}
			s.count++
			s.diagnosticQueue <- p.TextDocument.URI

		case lsp.MethodTextDocumentDidChange:
			var p lsp.DidChangeTextDocumentParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didChange: %v", err)
			}
			for _, c := range p.ContentChanges {
				lr := NewLineRange(c.Range)
				// Process each content change.
				nl := strings.Count(c.Text, `\n`)
				delta := int32(nl - int(lr.NumLines()))
				if delta == 0 {
					glog.V(1).Infof("No newline count change. Skipping update.")
					continue
				}
				s.MoveAnnotations(ctx, lr, delta, p.TextDocument.URI)
			}

		case lsp.MethodInitialized:
			if !s.gotInitialize {
				return fmt.Errorf("got initialized without initialize")
			}
			s.gotInitialized = true
			// Send some diagnostics here.
			close(s.initialized)
		case lsp.MethodShutdown:
			s.cancel()
			s.Shutdown()
		case lsp.MethodExit:
			if !s.gotShutdown {
				glog.Warningf("exiting without shutdown")
			}
			return ExitError // this will terminate the serving program.
		case lsp.MethodInitialize:
			var p lsp.InitializeParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				reply(ctx, jsonrpc2.NewError(jsonrpc2.ErrInternal.Code, ""), err)
				return fmt.Errorf("error during initialize: %v", err)
			}
			s.clientInfo = p.ClientInfo
			s.workspaceFolders = append(s.workspaceFolders, p.WorkspaceFolders...)
			// Result
			r := lsp.InitializeResult{
				ServerInfo: &lsp.ServerInfo{
					Name:    "pcc",
					Version: "0.0",
				},

				Capabilities: lsp.ServerCapabilities{
					//PositionEncoding: "utf-16",
					TextDocumentSync: &lsp.TextDocumentSyncOptions{
						OpenClose: true,
						Change:    lsp.TextDocumentSyncKindIncremental,
						//WillSave:  true,
						Save: &lsp.SaveOptions{
							//IncludeText: true,
						},
					},
					CodeLensProvider: &lsp.CodeLensOptions{
						// Have code lens, but no resolve provider.
						ResolveProvider: false,
					},
					Workspace: &lsp.ServerCapabilitiesWorkspace{
						FileOperations: &lsp.ServerCapabilitiesWorkspaceFileOperations{
							DidCreate: &lsp.FileOperationRegistrationOptions{
								Filters: []lsp.FileOperationFilter{
									{
										Pattern: lsp.FileOperationPattern{
											Glob: "*",
										},
									},
								},
							},
							//WillCreate: &lsp.FileOperationRegistrationOptions{},
							DidRename: &lsp.FileOperationRegistrationOptions{},
							//WillRename: &lsp.FileOperationRegistrationOptions{},
							DidDelete: &lsp.FileOperationRegistrationOptions{},
							//WillDelete: &lsp.FileOperationRegistrationOptions{},
						},
					},
				},
			}
			reply(ctx, r, nil)
			glog.V(1).Infof("Response: %v", spew.Sdump(r)) // This is expensive.
			s.gotInitialize = true
		default:
			reply(ctx, jsonrpc2.ErrMethodNotFound, nil)
		}
		return nil
	}
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

var ExitError = fmt.Errorf("exiting")

func Serve(f string, db *sql.DB) error {
	glog.Infof("listening for a connection at: %v", f)

	if f == defaultSocket {
		// Use a ReadWriteCloser from stdio and stout.
		jc := jsonrpc2.NewConn(jsonrpc2.NewStream(&StdioConn{}))
		ctx := context.Background()
		s, err := NewServer(ctx, db, jc)
		if err != nil {
			return fmt.Errorf("could not create server: %w", err)
		}
		srv := jsonrpc2.HandlerServer(s.GetHandlerFunc())
		if err := srv.ServeStream(ctx, jc); err != nil {
			if err != ExitError {
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

			s, err := NewServer(ctx, db, jc)
			if err != nil {
				return fmt.Errorf("could not create server: %w", err)
			}
			srv := jsonrpc2.HandlerServer(s.GetHandlerFunc())
			// Probably needs to be in a separate goroutine, this.
			if err := srv.ServeStream(ctx, jc); err != nil {
				glog.Infof("error while serving request: %v", err)
				if err == ExitError {
					break
				}
			}
		}
	}
	return nil
}
