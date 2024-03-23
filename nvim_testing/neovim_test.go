package nvim_testing

import (
	"context"
	"flag"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/filmil/private-code-comments/pkg"
	"github.com/filmil/private-code-comments/tc"
	_ "github.com/mattn/go-sqlite3"
	"github.com/neovim/go-client/msgpack/rpc"
	lsp "go.lsp.dev/protocol"
)

var (
	editFile = flag.String("edit-file", "", "")
	ws       lsp.URI
)

func init() {
	ws = "ws" // from pcc.config.json
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

	db, closeFn := tc.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	tc.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := tc.Must(NewNeovim(dbFile))

	c := tc.Must(GetLspAttachEvent(n, "*"))

	tc.Must1(EditFile(n, NotEmpty(*editFile)))

	<-c

	buf := tc.Must(n.CurrentBuffer())

	// Not sure why this must be done. But if it isn't, then the write won't
	// get seen by nvim.
	tc.Must(pkg.GetAnns(db, string(ws), testFilename))

	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	// Insert some text at line 1, see what happened.
	tc.Must1(InsertText(n, buf, 0, "add new at 1: hello\n"))

	LogAllLines(t, tc.Must(GetAllLines(n, buf)))
	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 11, Content: "hello!"},
	}))
}

func TestDeleteLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

	db, closeFn := tc.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	tc.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := tc.Must(NewNeovim(dbFile))

	c := tc.Must(GetLspAttachEvent(n, "*"))

	tc.Must1(EditFile(n, NotEmpty(*editFile)))

	<-c

	buf := tc.Must(n.CurrentBuffer())

	// If we don't wait, we might get a didOpen with modified content, which
	// we don't really want.
	tc.Must1(WaitForLine(ctx, n, buf, 0, "     1\tSome text."))

	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	tc.Must1(RemoveTextLines(n, buf, 0, 1))
	LogAllLines(t, tc.Must(GetAllLines(n, buf)))

	tc.Must1(WaitForLine(ctx, n, buf, 0, "     2\tSome text."))

	// Surprise! the transferred change is a minimal edit.
	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 9, Content: "hello!"},
	}))
}

func TestGetLine(t *testing.T) {
	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

	db, closeFn := tc.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()
	tc.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := tc.Must(NewNeovim(dbFile))

	e := tc.Must(GetLspAttachEvent(n, "*"))

	// Must edit the file *after* the event setup is done.
	tc.Must1(n.Command(fmt.Sprintf("edit %s", *editFile)))

	// Must wait strictly *after* an event that will produce the event.
	<-e

	tc.Must1(SetComment(n, "Hello note!"))

	tc.Must1(WaitForAnn(context.TODO(), n, "Hello note!"))

	n.Command("quit")
}

func TestDeleteSingletonLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := BazelTmpDir(t)
	dbFile := path.Join(tmpDir, dbName(t))

	db, closeFn := tc.Must3(RunDBQuery(dbFile, ``))
	defer closeFn()

	tc.Must1(pkg.InsertAnn(db, string(ws), testFilename, 10, "hello!"))
	n := tc.Must(NewNeovim(dbFile))

	c := tc.Must(GetLspAttachEvent(n, "*"))

	tc.Must1(EditFile(n, NotEmpty(*editFile)))

	<-c

	buf := tc.Must(n.CurrentBuffer())

	// Ensure there are annotations.
	tc.Must1(WaitForLine(ctx, n, buf, 0, "     1\tSome text."))
	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{
		{Line: 10, Content: "hello!"},
	}))

	tc.Must1(MoveToLine(n, 10))
	note1 := tc.Must(GetComment(n))
	if note1 != "hello!" {
		t.Errorf("expected note, but found: %q", note1)
	}
	tc.Must1(DeleteCommentAtCurrentLine(n))
	tc.Must1(WaitForAnns(ctx, db, ws, testFilename, []pkg.Ann{}))
	note := tc.Must(GetComment(n))
	if note != "" {
		t.Errorf("expected no note, but found: %q", note)
	}
}
