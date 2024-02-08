package nvim_testing

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/filmil/private-code-comments/pkg"
	"github.com/golang/glog"
	"github.com/neovim/go-client/nvim"
	lsp "go.lsp.dev/protocol"
)

var instance int

func getInstance() int {
	instance++
	return instance
}

var (
	pccBinary    = flag.String("pcc-binary", "", "")
	pluginVimDir = flag.String("plugin-nvim-dir", "", "")
	nvimShareDir = flag.String("nvim-share-dir", "", "")
	nvimLibDir   = flag.String("nvim-lib-dir", "", "")
	nvimBinary   = flag.String("nvim-binary", "", "")
	nvimLuaDir   = flag.String("nvim-lua-dir", "", "")
)

func NotEmpty(s string) string {
	if s == "" {
		panic("env variable should not be empty - are you sure you are using nvim_go_test target?")
	}
	return s
}

func BazelTmpDir(t *testing.T) string {
	var ret string
	ret = os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR")
	if ret == "" {
		if t == nil {
			panic("can not generate a tmpdir without bazel or test")
		}
		ret = t.TempDir()
	}
	return ret
}

// NewNeovim creates a new Neovim child process for testing.
//
// The created Neovim is a hermetic instance.
func NewNeovim(dbfile string, args ...string) (*nvim.Nvim, error) {
	outDir := NotEmpty(BazelTmpDir(nil))
	i := getInstance()
	pccLogDir, err := os.MkdirTemp(outDir, fmt.Sprintf("%03d-log.dir", i))
	logFile := path.Join(pccLogDir, "neovim-log")
	args = append([]string{
		"--embed",
		"--headless",
		fmt.Sprintf("-V10%v", logFile),
	},
		args...)
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	return nvim.NewChildProcess(nvim.ChildProcessEnv(
		[]string{
			// Set up a hermetic environment, with local dirs.
			"USERNAME=unknown",
			"LOGNAME=unknown",
			fmt.Sprintf("PCC_LOG_DIR=%v", NotEmpty(pccLogDir)),
			fmt.Sprintf("PCC_DB=%v", NotEmpty(dbfile)),
			fmt.Sprintf("PCC_BINARY=%v", NotEmpty(*pccBinary)),
			fmt.Sprintf("XDG_CONFIG_HOME=%v", NotEmpty(*nvimLuaDir)),
			// This is where neovim logs will go.
			fmt.Sprintf("XDG_STATE_HOME=%v", NotEmpty(pccLogDir)),
			fmt.Sprintf("XDG_CONFIG_DIRS=%v:%v",
				NotEmpty(*nvimLuaDir), NotEmpty(*pluginVimDir)),
			fmt.Sprintf("VIMRUNTIME=%v", NotEmpty(*nvimShareDir)),
			fmt.Sprintf("LD_PRELOAD_PATH=%v", NotEmpty(*nvimLibDir)),
		}),
		// Use our own Neovim executable.
		nvim.ChildProcessCommand(NotEmpty(*nvimBinary)),
		// And pass some args in.
		nvim.ChildProcessArgs(args...),
		nvim.ChildProcessLogf(glog.Infof),
		nvim.ChildProcessServe(true),
	)
}

// WaitForNsBuf waits for the namespace with `name` to appear in the nvim client `cl`.
func WaitForNsBuf(ctx context.Context, cl *nvim.Nvim, buf nvim.Buffer, name string) (int, error) {
	ns := fmt.Sprintf("%s.%d", name, buf)
	return WaitForNs(ctx, cl, ns)
}

// WaitForNs waits for the namespace with `name` to appear in the nvim client `cl`.
func WaitForNs(ctx context.Context, cl *nvim.Nvim, ns string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		nss, err := cl.Namespaces()
		if err != nil {
			return -1, fmt.Errorf("could not get namespaces.")
		}
		v, ok := nss[ns]
		if ok {
			return v, nil
		}
		select {
		case <-ctx.Done():
			return -1, fmt.Errorf("WaitForNs: timeout: ns=%v", ns)
		case <-time.After(1 * time.Second):
		}
	}
}

// WaitForAnns waits for annotations.  Returns error if the exact annotations are
// not found.
func WaitForAnns(ctx context.Context, db *sql.DB, ws lsp.URI, file string, anns []pkg.Ann) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var i int
	for {
		actual, err := pkg.GetAnns(db, string(ws), file)
		if err != nil {
			return fmt.Errorf("could not get anns: %w", err)
		}
		if reflect.DeepEqual(anns, actual) {
			return nil
		}
		i++
		select {
		case <-ctx.Done():
			return fmt.Errorf("WaitForAnns: timeout: got=%+v, want: %+v", actual, anns)
		case <-time.After(1 * time.Second):
		}
	}
}

func InsertText(cl *nvim.Nvim, buf nvim.Buffer, line int, text string) error {
	strs := strings.Split(text, "\n")
	bytes := [][]byte{}
	for _, l := range strs {
		bytes = append(bytes, []byte(l))
	}
	return cl.SetBufferText(buf, line, 0, line, 0, bytes)
}

func RemoveTextLines(cl *nvim.Nvim, buf nvim.Buffer, begin, count int) error {
	ret := cl.SetBufferText(buf, begin, 0, begin+count, 0, [][]byte{{}})
	return ret
}

func GetAllLines(cl *nvim.Nvim, buf nvim.Buffer) ([]string, error) {
	l, err := cl.BufferLines(buf, 0, -1, false)
	if err != nil {
		return []string{}, fmt.Errorf("could not get lines: %w", err)
	}
	var ret []string
	for _, b := range l {
		ret = append(ret, string(b))
	}
	return ret, nil
}

func LogAllLines(t *testing.T, lines []string) {
	fmt.Printf("====: %v\n", t.Name())
	for i, ln := range lines {
		fmt.Printf("line:%03d: %q\n", i, ln)
	}
}

// WaitForAnns waits for annotations.  Returns error if the exact annotations are
// not found.
func WaitForLine(ctx context.Context, cl *nvim.Nvim, buf nvim.Buffer, line int, text string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		actual, err := GetAllLines(cl, buf)
		if err != nil {
			return fmt.Errorf("could not get lines: %w", err)
		}
		if len(actual) > 0 && len(actual) > line {
			if actual[line] == text {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("WaitForLine: timeout: got=%q, want: %q", actual[line], text)
		case <-time.After(1 * time.Second):
		}
	}
}

// GetLspAttachEvent returns a channel that is closed when a LspAttach event
// happens.  This is the only go-client example for handling this that I am aware of.
// For details, see: https://github.com/neovim/neovim/discussions/27371
//
// GetLspAttachEvent configures the Neovim client `cl` to close the returned channel
// when it gets a `LspAttach` event.  It seems that there is no automated way to connect
// nvim events such that the notifications to a go client happen automatically. Instead,
// we configure it ourselves.
//
// Other subtleties are that you must ensure that you get the channel here strictly
// *before* sending any commands that could result in this notification. Otherwise,
// you can sometimes *miss* that event.  For example, if you load a file from command
// line in Neovim, you might miss most, or all of the events emitted as a result of
// opening that file.
//
// Args:
//   - cl: a neovim client.  Get one from `nvim.NewChildProcess`, for example.
//   - pattern: the autocmd pattern, for example, "text", or "*".
//
// Returns: a channel that gets closed when the event is received.
func GetLspAttachEvent(cl *nvim.Nvim, pattern string) (chan struct{}, error) {
	const eName = `LspAttach`
	// The name in Subscribe and Unsubscribe
	if err := cl.Subscribe(eName); err != nil {
		return nil, fmt.Errorf("failed to subscribe to event: %w", err)
	}
	var (
		// The returned value is the ID of the registerd autocmd.
		id   int
		args struct{}
	)
	if err := cl.ExecLua(fmt.Sprintf(`
        -- without a return here, you will not get your code executed apparently.
        return vim.api.nvim_create_autocmd(
            '%s', 
            {
                callback = function(e)
                    -- You'd expect to use output of vim.nvim_get_api_info()[1]
                    -- here, but I could not get that function to return something
                    -- other than nil.
                    --
                    -- The second parameter here is the RPC method name which
                    -- is the argument of Subscribe above, and Unsubscribe below.
                    vim.rpcnotify(0, '%s')
                end,
                -- For file.txt, the pattern must apparently be either '*' or
                -- 'text', else you will not get a notification it seems.
                pattern = { '%s' },
                nested = true,
            }
        )
    `, eName, eName, pattern), &id, &args); err != nil {
		return nil, fmt.Errorf("could not ")
	}

	c := make(chan struct{})
	err := cl.RegisterHandler(eName, func(cl *nvim.Nvim, a any) error {
		defer close(c)
		// This is the same name as in the call to `cl.Subscribe`.
		cl.Unsubscribe(eName)
		var (
			// A nil result serializes to interface{}
			result interface{}
			args   struct{}
		)
		err := cl.ExecLua(fmt.Sprintf(`return vim.api.nvim_del_autocmd(%d)`, id), &result, &args)
		if err != nil {
			return fmt.Errorf("could not delete autocmd: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not register handler: %w", err)
	}
	return c, nil
}
