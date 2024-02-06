package pkg

import (
	"fmt"
	"strings"

	lsp "go.lsp.dev/protocol"
)

// RPath returns a file path relative to the given workspace.
// ws is the string representation of a workspace URI, e.g. "file:///ws"
// fileURI is the an URI possibly in that workspace, like "file:///ws/file.txt"
// In this case we'll be returning "/file.txt", i.e. a path rooted in `ws`.
func RPath(ws string, fileURI lsp.URI) string {
	f := string(fileURI)
	if !strings.HasPrefix(f, ws) {
		panic(fmt.Sprintf("ws is not a prefix: ws=%q, file=%v", ws, fileURI))
	}
	return strings.TrimPrefix(string(fileURI), ws)
}

// Finds the workspace that the file with uri URI belongs to.
// Returns the workspace URI encoded as string, and the relative
// path for the provided file.
//
// Example:
//
//	For a workspace "file:///ws/file.txt", returns:
//	("file:///ws", "/file.txt")
func FindWorkspace(w []lsp.WorkspaceFolder, fileURI lsp.URI) (string, string) {
	u := string(fileURI)
	if !strings.HasPrefix(u, "file://") {
		panic(fmt.Sprintf("no `file` scheme in URI: %v", u))
	}
	var p, f string
	for _, ws := range w {
		if strings.HasPrefix(u, ws.URI) && len(ws.URI) > len(p) {
			f = RPath(ws.URI, fileURI)
			p = ws.URI
		}
	}
	return p, f
}
