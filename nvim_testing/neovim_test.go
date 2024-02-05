package nvim_testing

import (
	"flag"
	"testing"

	"github.com/neovim/go-client/msgpack/rpc"
)

var (
	editFile = flag.String("edit-file", "", "")
)

func TestOne(t *testing.T) {
	e := NotEmpty(*editFile)
	n, err := NewNeovim(e)
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
