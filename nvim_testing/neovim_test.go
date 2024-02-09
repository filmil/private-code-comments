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

func init() {
	wsx, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not get working dir: %v", err))
	}
	ws = lsp.URI(fmt.Sprintf("file://%v", filepath.Clean(wsx)))
}

func TestOne(t *testing.T) {
	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

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

func dbName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%v.db.sqlite", t.Name())
}

func TestInsertLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

	db, closeFn := pkg.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	pkg.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := pkg.Must(NewNeovim(dbFile))

	c := pkg.Must(GetLspAttachEvent(n, "*"))

	pkg.Must1(EditFile(n, NotEmpty(*editFile)))

	<-c

	buf := pkg.Must(n.CurrentBuffer())

	// Not sure why this pkg.Must be done. But if it isn't, then the write won't
	// get seen by nvim.
	pkg.Must(pkg.GetAnns(db, string(ws), testFilename))

	pkg.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	// Insert some text at line 1, see what happened.
	pkg.Must1(InsertText(n, buf, 0, "add new at 1: hello\n"))

	LogAllLines(t, pkg.Must(GetAllLines(n, buf)))
	pkg.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 11, Content: "hello!"},
	}))
}

func TestDeleteLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

	db, closeFn := pkg.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	pkg.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := pkg.Must(NewNeovim(dbFile))

	c := pkg.Must(GetLspAttachEvent(n, "*"))

	pkg.Must1(EditFile(n, NotEmpty(*editFile)))

	<-c

	buf := pkg.Must(n.CurrentBuffer())

	// If we don't wait, we might get a didOpen with modified content, which
	// we don't really want.
	pkg.Must1(WaitForLine(ctx, n, buf, 0, "     1\tSome text."))

	pkg.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	pkg.Must1(RemoveTextLines(n, buf, 0, 1))
	LogAllLines(t, pkg.Must(GetAllLines(n, buf)))

	pkg.Must1(WaitForLine(ctx, n, buf, 0, "     2\tSome text."))

	// Surprise! the transferred change is a minimal edit.
	pkg.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 9, Content: "hello!"},
	}))
}

//func TestGetLine(t *testing.T) {
//tmpDir := BazelTmpDir(t)
//dbFile := path.Join(tmpDir, dbName(t))

//db, closeFn := pkg.Must3(RunDBQuery(dbFile, ``))
//defer closeFn()
//pkg.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
//n := pkg.Must(NewNeovim(dbFile))

//e := pkg.Must(GetLspAttachEvent(n, "*"))

//// Must edit the file *after* the event setup is done.
//pkg.Must1(n.Command(fmt.Sprintf("edit %s", *editFile)))

//// Must wait strictly *after* an event that will produce the event.
//<-e

//var (
//result string
//args   struct{}
//)

//pkg.Must1(n.ExecLua(
//`return require('pcc').get_comment()`,
//&result,
//&args))
//pkg.Must1(n.Command("quit"))
//if result != "" {
//t.Errorf("want: %v, got: %v", "", result)
//}
//}
