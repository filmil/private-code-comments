# Private Code Comments LSP [![Test status](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)

## TODO

* [ ] Figure out how to get a callback for plugin.HandleAutocmd.
  * [ ] Figure out why the tests are flaky, probably related to above.
    * [ ] LspAttach never arrives.

## Maintenance

Run problem tests:

```bash
bazel test //nvim_testing/...
```

View the log results:

```
unzip -c  bazel-testlogs/nvim_testing/nvim_testing_test/test.outputs/outputs.zip | less
```

## How to  configure neovim to run this LSP.

```lua
vim.lsp.set_log_level("debug")
local lspconfig = require 'lspconfig'
local configs = require 'lspconfig.configs'

if not configs.pcc then
  configs.pcc = {
    default_config = {
      name = 'pcc',
      cmd = {
        '/home/fmil/code/private-code-comments/pcc',
        '--log_dir=/home/fmil/tmp',
        '--v=3',
      },
      root_dir = lspconfig.util.root_pattern('go.mod'),
      filetypes = { "text" },
      },
  }
end
lspconfig.pcc.setup {}
```

