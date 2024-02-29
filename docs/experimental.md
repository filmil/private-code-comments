# Initializing pcc with a lsp-config based setup.

Noting this down here, so I don't forget.

```
-- XXX(fmil): PCC setup - experimental.

local lspconfig = require 'lspconfig'
local configs = require 'lspconfig.configs'
local pcc_plugin = require('pcc')

if not configs.pcc then
  configs.pcc = {
    default_config = {
      name = 'pcc',
      cmd = {
        os.getenv("HOME") .. '/.local/bin/pcc',
        '--log_dir=' .. os.getenv("HOME") .. '/.local/state/pcc/logs',
        '--v=3',
        '--db=' .. os.getenv("HOME") .. '/.local/state/pcc/db/db.sqlite',
      },
      root_dir = lspconfig.util.root_pattern({ ".git", "pcc.config.json" }),
      filetypes = { "text", "lua", "rust", "gn" },
      handlers = pcc_plugin.handlers(),
    },
  }
end
lspconfig.pcc.setup {}

pcc_plugin.config = {
    annot_win_width = 25,
    annot_win_padding = 2,
}
vim.keymap.set({'n'}, '<leader>cr', pcc_plugin.edit)
vim.keymap.set({'n'}, '<leader>cd', pcc_plugin.delete)
```
