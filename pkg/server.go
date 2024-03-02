package pkg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
)

var ExitError = fmt.Errorf("exiting")

// DiagnosticMsg contains the message sent to the diagnostic hander.
type DiagnosticMsg struct {
	// The URI of the file to refresh.
	URI lsp.URI

	// If set, the diagnostics are always updated. When unset, the Diagnostic
	// update is allowed to skip some updates.
	Force bool
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
	diagnosticQueue chan DiagnosticMsg
	globalCtx       context.Context
	db              *sql.DB
	cancel          context.CancelFunc

	// Just a temporary thing.
	count int
}

// Finds the workspace that the file with uri URI belongs to.
// Returns the workspace URI encoded as string, and the relative
// path for the provided file.
func (s *Server) FindWorkspace(fileURI lsp.URI) (string, string) {
	return FindWorkspace(s.workspaceFolders, fileURI)
}

func NewServer(ctx context.Context, db *sql.DB, conn jsonrpc2.Conn) (*Server, error) {
	// Initialize the database.
	ctx, cancel := context.WithCancel(ctx)

	s := Server{
		// The queue capacity needs to be a little bit large, since the processing function
		// may add to it.
		diagnosticQueue: make(chan DiagnosticMsg, 10),
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
	End              uint32
	StartCol, EndCol uint32
}

// NewLineRange creates a unified LineRange from equivalent LSP type.
func NewLineRange(r lsp.Range) LineRange {
	return LineRange{
		Start:    r.Start.Line,
		End:      r.End.Line,
		StartCol: r.Start.Character,
		EndCol:   r.End.Character,
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

func (s *Server) DiagnosticsFn() {
	<-s.initialized
	glog.V(1).Infof("diagnostics up and running")
	defer func() {
		glog.V(1).Infof("diagnostics is exiting")
	}()
	ctx, cancel := context.WithCancel(s.globalCtx)
	defer cancel()
	for {
		glog.Flush()
		select {
		case <-s.globalCtx.Done():
			break
		case q := <-s.diagnosticQueue:
			uri := q.URI
			glog.V(1).Infof("diagnosticFn: command: %+v", q)
			ws, rpath := s.FindWorkspace(uri)
			glog.V(4).Infof("Operating on ws=%q, path=%q for: %v", ws, rpath, uri)
			anns, err := GetAnns(s.db, ws, rpath)
			if err != nil {
				glog.Errorf("error getting annotations: workspace=%v, file=%v: %v", ws, rpath, err)
			}
			if len(anns) == 0 && !q.Force {
				glog.V(1).Infof("DiagnosticsFn: nothing to publish.")
				continue
			}
			// This will delete diagnostics when not present.
			d := []lsp.Diagnostic{}
			for _, a := range anns {
				d = append(d, MakeDiagnostic(
					LineRange{Start: a.Line, End: a.Line + 1}, a.Content))
			}
			p := lsp.PublishDiagnosticsParams{URI: uri, Diagnostics: d}
			glog.V(2).Infof("publishing diagnostics: %s", spew.Sdump(p))
			if err := s.conn.Notify(
				ctx, lsp.MethodTextDocumentPublishDiagnostics, &p); err != nil {
				glog.Errorf("DiagnosticsFn: error while publishing diagnostics for: %v: %v", uri, err)
			}
		}
	}
}

// INVARIANT: delta != 0.
func (s *Server) MoveAnnotations(ctx context.Context, lr LineRange, delta int32, uri lsp.URI) error {
	if delta == 0 {
		// We already excluded delta==0 when calling here.
		glog.Fatalf("delta==0: this should not happen")
	}

	// When lines are inserted, we increment the line number of all annotations from the
	// line after the insert line by the number of inserted lines.
	ws, rpath := s.FindWorkspace(uri)

	if delta > 0 {
		if err := BulkMoveAnn(s.db, ws, rpath, lr.Start, delta); err != nil {
			return fmt.Errorf("MoveAnnotations: %w", err)
		}
	}
	if delta < 0 {
		// Deletion is a tad bit complicated.
		// (1) The lines below the delete are moved up by delta.
		// (2) The lines affected by the delete, are merged in sequence, and
		// attached to the first line. As a transaction.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("could not begin tx")
		}
		err = TxBulkAppendAnn(tx, ws, rpath, lr.Start, lr.End, delta)
		if err != nil {
			return fmt.Errorf("MoveAnnotations: %w", err)
		}
		if err := tx.Commit(); err != nil {
			glog.Errorf("could not commit: %v", err)
		}
	}

	glog.V(1).Info("refresh diagnostics.")
	s.diagnosticQueue <- DiagnosticMsg{URI: uri}
	return nil
}

const (
	PccSetCmd = `$/pcc/set`
	PccGetCmd = `$/pcc/get`
	CancelCmd = `%/cancelRequest`
)

// GetHandlerFunc returns a stateful function that can be given to jsonrpc2.StreamServer
// to serve JSON-RPC2 requests.
func (s *Server) GetHandlerFunc() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		glog.Infof("JSON-RPC2 Request method: %v", req.Method())
		defer func() {
			glog.Flush()
		}()

		switch req.Method() {
		case lsp.MethodCancelRequest:
			glog.Infof("JSON-RPC2: cancel: %+v", string(req.Params()))
		case PccGetCmd:
			glog.Infof("JSON-RPC2: %+v", string(req.Params()))
			var p PccGet
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during $/pcc/get: %w", err)
			}
			glog.V(1).Infof(PccGetCmd+": Request: %v", spew.Sdump(p)) // This is expensive.
			// Sanity check here.

			if !strings.HasPrefix(string(p.File), "file:") {
				return fmt.Errorf("malformed file URI, no scheme: %+v", p)
			}
			ws, rpath := FindWorkspace(s.workspaceFolders, p.File)
			ann, err := GetAnn(s.db, ws, rpath, p.Line)
			if err != nil {
				return fmt.Errorf("could not get annotation: %+v: %w", p, err)
			}
			r := PccGetResp{
				Content: strings.Split(ann, "\n"),
			}
			glog.V(3).Infof(PccGetCmd+": reply: %v", spew.Sdump(r))
			return reply(ctx, r, nil)

		case PccSetCmd:
			var p PccSet
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during $/pcc/get: %v", err)
			}
			glog.V(3).Infof(PccSetCmd+": Request: %v", spew.Sdump(p)) // This is expensive.
			ws, rpath := FindWorkspace(s.workspaceFolders, p.File)
			content := strings.Join(p.Content, "\n")
			force := false
			if content == "" {
				if err := DeleteAnn(s.db, ws, rpath, p.Line); err != nil {
					err := fmt.Errorf("could not delete: %+v: %w", p, err)
					glog.V(1).Infof(PccSetCmd+": error: %v", err)
					return err
				}
				force = true
			} else {
				// Update.
				if err := InsertAnn(s.db, ws, rpath, p.Line, content); err != nil {
					err := fmt.Errorf("could not upsert: %+v: %w", p, err)
					glog.V(1).Infof(PccSetCmd+": error: %v", err)
					return err
				}
			}
			reply(ctx, PccSetRes{}, nil)
			s.diagnosticQueue <- DiagnosticMsg{URI: p.File, Force: force}

		case lsp.MethodTextDocumentDidSave:
			var p lsp.DidSaveTextDocumentParams
			glog.V(1).Infof("didSave: Request: %v", spew.Sdump(p)) // This is expensive.
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didSave: %v", err)
			}
		case lsp.MethodTextDocumentDidOpen:
			var p lsp.DidOpenTextDocumentParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didOpen: %v", err)
			}
			glog.V(1).Infof("didOpen: Request: %v", spew.Sdump(p)) // This is expensive.
			s.count++
			s.diagnosticQueue <- DiagnosticMsg{URI: p.TextDocument.URI}

		case lsp.MethodTextDocumentDidChange:
			var p lsp.DidChangeTextDocumentParams
			if err := json.Unmarshal(req.Params(), &p); err != nil {
				return fmt.Errorf("error during didChange: %v", err)
			}
			glog.V(1).Infof("didChange: Request: %v", spew.Sdump(p)) // This is expensive.
			for _, c := range p.ContentChanges {
				lr := NewLineRange(c.Range)
				// Process each content change.
				nl := strings.Count(c.Text, "\n")
				delta := int32(nl - int(lr.NumLines()))
				if delta == 0 {
					glog.V(1).Infof("No newline count change. Skipping update: lr=%+v, nl=%v", lr, nl)
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
			glog.V(1).Infof("Request: %v", spew.Sdump(p)) // This is expensive.
			s.clientInfo = p.ClientInfo
			s.workspaceFolders = ResolveWs(append(s.workspaceFolders, p.WorkspaceFolders...))
			glog.V(1).Infof("workspaces: %+v", s.workspaceFolders)
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
