-- would have been better if I could add from a command line, but apparently that
-- does not work.
-- Plugin setup.

-- From //nvim_testing/content:workspace.marker
local root_patterns = { "workspace.marker" }
local log_cmd = "--log_dir=" .. os.getenv("PCC_LOG_DIR")

vim.lsp.set_log_level("debug")
vim.api.nvim_create_autocmd(
  { "FileType" },
  {
    pattern = "text",
    callback = function()
      print("entering txt file\n")
      vim.lsp.start({
	cmd = {
	    os.getenv("PCC_BINARY"),
	    log_cmd,
	    "--v=4",
	    "--db=" .. os.getenv("PCC_DB"),
	},
	root_dir = vim.fs.dirname(
	  vim.fs.find(root_patterns, { upward = true })[1]),
      })
    end
  }
)

require('pcc').setup()
