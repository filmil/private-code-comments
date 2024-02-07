package nvim_testing

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/filmil/private-code-comments/pkg"
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
	args = append([]string{
		"--embed",
		"--headless",
	},
		args...)
	outDir := NotEmpty(BazelTmpDir(nil))
	i := getInstance()
	pccLog, err := os.MkdirTemp(outDir, fmt.Sprintf("%03d-pcc-", i))
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	neovimLog, err := os.MkdirTemp(outDir, fmt.Sprintf("%03d-neovim-", i))
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	return nvim.NewChildProcess(nvim.ChildProcessEnv(
		[]string{
			// Set up a hermetic environment, with local dirs.
			"USERNAME=unknown",
			"LOGNAME=unknown",
			fmt.Sprintf("PCC_LOG_DIR=%v", NotEmpty(pccLog)),
			fmt.Sprintf("PCC_DB=%v", NotEmpty(dbfile)),
			fmt.Sprintf("PCC_BINARY=%v", NotEmpty(*pccBinary)),
			fmt.Sprintf("XDG_CONFIG_HOME=%v", NotEmpty(*nvimLuaDir)),
			// This is where neovim logs will go.
			fmt.Sprintf("XDG_STATE_HOME=%v", NotEmpty(neovimLog)),
			fmt.Sprintf("XDG_CONFIG_DIRS=%v:%v",
				NotEmpty(*nvimLuaDir), NotEmpty(*pluginVimDir)),
			fmt.Sprintf("VIMRUNTIME=%v", NotEmpty(*nvimShareDir)),
			fmt.Sprintf("LD_PRELOAD_PATH=%v", NotEmpty(*nvimLibDir)),
		}),
		// Use our own Neovim executable.
		nvim.ChildProcessCommand(NotEmpty(*nvimBinary)),
		// And pass some args in.
		nvim.ChildProcessArgs(args...),
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
