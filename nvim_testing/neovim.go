package nvim_testing

import (
	"flag"
	"fmt"
	"os"

	"github.com/neovim/go-client/nvim"
)

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

// NewNeovim creates a new Neovim child process for testing.
//
// The created Neovim is a hermetic instance.
func NewNeovim(args ...string) (*nvim.Nvim, error) {
	args = append([]string{"--embed", "--headless"}, args...)
	return nvim.NewChildProcess(nvim.ChildProcessEnv(
		[]string{
			// Set up a hermetic environment, with local dirs.
			"USERNAME=unknown",
			"LOGNAME=unknown",
			fmt.Sprintf("HOME=%v", os.Getenv("TEST_TMPDIR")),
			fmt.Sprintf("PCC_BINARY=%v", NotEmpty(*pccBinary)),
			fmt.Sprintf("XDG_CONFIG_HOME=%v", NotEmpty(*nvimLuaDir)),
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
