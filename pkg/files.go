package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"
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
	var pWs, pFile string
	for _, ws := range w {
		if strings.HasPrefix(u, ws.URI) && len(ws.URI) > len(pWs) {
			pFile = RPath(ws.URI, fileURI)
			if ws.Name != "" {
				pWs = ws.Name
			} else {
				pWs = ws.URI
			}
		}
	}
	return pWs, pFile
}

const ConfigFilename = `pcc.config.json`

// ResolveWs resolves the workspace names, potentially using the marker config
// filename to get the workspace name.
//
// For a workspace: "file:///ws/file.txt"
// and a workspace name mapping: "file:///ws" -> "ws"
// returns:
//
// ("ws", "/file.txt")
func ResolveWs(in []lsp.WorkspaceFolder) []lsp.WorkspaceFolder {
	for i := range in {
		ws := lsp.URI(in[i].URI)
		cfg := path.Join(ws.Filename(), ConfigFilename)
		_, err := os.Stat(cfg)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				glog.Warningf("could not read config from: %v: %v", cfg, err)
			}
			continue
		}
		f, err := os.Open(cfg)
		if err != nil {
			glog.Warningf("could not open config from: %v, %v", cfg, err)
			continue
		}
		c, err := JSONUnmarshal[WorkspaceConfig](f)
		if err != nil {
			glog.Warningf("could not parse config from: %v: %v", cfg, err)
			continue
		}
		in[i].Name = c.WorkspaceName
	}
	return in
}

// JSONUnmarshal is a typed parser for a go type.
func JSONUnmarshal[T any](r io.Reader) (T, error) {
	var ret T
	d := json.NewDecoder(r)
	if err := d.Decode(&ret); err != nil {
		return ret, fmt.Errorf("could not unmarshal: %T: %w", ret, err)
	}
	return ret, nil
}
