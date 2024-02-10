-- From //nvim_testing/content:workspace.marker
local root_patterns = { "workspace.marker" }

local log_cmd = "--log_dir=" .. os.getenv("PCC_LOG_DIR")

require('pcc').setup()

vim.lsp.set_log_level("debug")
vim.api.nvim_create_autocmd(
  { "FileType" },
  {
    pattern = { "text" },
    nested = true,
    callback = function()
      vim.lsp.start({
	cmd = {
	    os.getenv("PCC_BINARY"),
	    log_cmd,
	    "--v=4",
	    "--db=" .. os.getenv("PCC_DB"),
	},
	root_dir = vim.fs.dirname(
	  vim.fs.find(root_patterns, { upward = true })[1]),

	handlers = require('pcc').handlers(),
      })
    end
  }
)

