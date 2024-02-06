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

func must3[T any, V any](t T, v V, err error) (T, V) {
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
	return t, v
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

//func TestInsertLine(t *testing.T) {
//ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//defer cancel()

//tmpDir := t.TempDir()
//dbFile := path.Join(tmpDir, "db.sqlite")

//db, closeFn := must3(RunDBQuery(dbFile, ``))
//defer closeFn()

//must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
//n := must(NewNeovim(dbFile, NotEmpty(*editFile)))

//buf := must(n.CurrentBuffer())

//// Not sure why this must be done. But if it isn't, then the write won't
//// get seen by nvim.
//must(pkg.GetAnns(db, string(ws), testFilename))

//must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
//{Line: 10, Content: "hello!"},
//}))

//// Insert some text at line 1, see what happened.
//must1(InsertText(n, buf, 0, "add new at 1: hello\n"))

//LogAllLines(t, must(GetAllLines(n, buf)))
//must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
//{Line: 11, Content: "hello!"},
//}))
//}

func TestDeleteLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	dbFile := path.Join(tmpDir, "db.sqlite")

	db, closeFn := must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := must(NewNeovim(dbFile, NotEmpty(*editFile)))

	buf := must(n.CurrentBuffer())

	// If we don't wait, we might get a didOpen with modified content, which
	// we don't really want.
	must1(WaitForLine(ctx, n, buf, 0, "     1\tSome text."))

	must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	must1(RemoveTextLines(n, buf, 0, 1))
	LogAllLines(t, must(GetAllLines(n, buf)))

	must1(WaitForLine(ctx, n, buf, 0, "     2\tSome text."))

	// Surprise! the transferred change is a minimal edit.
	must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 9, Content: "hello!"},
	}))
}
