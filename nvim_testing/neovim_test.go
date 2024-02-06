package nvim_testing

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/filmil/private-code-comments/pkg"
	_ "github.com/mattn/go-sqlite3"
	"github.com/neovim/go-client/msgpack/rpc"
	lsp "go.lsp.dev/protocol"
)

var (
	editFile = flag.String("edit-file", "", "")
	ws       lsp.URI
)

func must1(err error) {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
}
func must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
	return v
}

func init() {
	wsx, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working dir: %v", err))
	}
	ws = lsp.URI(fmt.Sprintf("file://%v", filepath.Clean(wsx)))
}

func TestOne(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := path.Join(tmpDir, "db.sqlite")

	e := NotEmpty(*editFile)
	n, err := NewNeovim(NotEmpty(dbFile), e)
	if err != nil {
		t.Errorf("could not start hermetic neovim: %v", err)
	}
	if err := n.Command("quit"); err != nil {
		// rpc.ErrClosed is a response to a `quit` command.
		if err != rpc.ErrClosed {
			t.Errorf("could not quit neovim: %v, %#v", err, err)
		}
	}
}

var withDetails = map[string]interface{}{
	"details": true,
	"limit":   100,
}

const testFilename = "/nvim_testing/content/textfile.txt"

func TestInsertLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	dbFile := path.Join(tmpDir, "db.sqlite")

	db, closeFn, err := RunDBQuery(dbFile, ``)
	if err != nil {
		t.Fatalf("could not create database: %v: %v", dbFile, err)
	}
	defer closeFn()
	must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := must(NewNeovim(dbFile, NotEmpty(*editFile)))
	must(n.BufferLineCount(0))
	must(n.Namespaces())

	buf := must(n.CurrentBuffer())
	must(n.BufferLines(buf, 0, -1, true))
	must(WaitForNsBuf(ctx, n, buf, "vim.lsp.pcc"))
	must(pkg.GetAnns(db, string(ws), testFilename))
	must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	// Insert some text at line 1, see what happened.
	must1(InsertText(n, buf, 0, "hello\n"))

	must(n.BufferLines(buf, 0, -1, true))
	must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 11, Content: "hello!"},
	}))
}
