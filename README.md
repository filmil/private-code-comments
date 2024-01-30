# Private Code Comments LSP [![Test status](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)

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
