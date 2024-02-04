-- would have been better if I could add from a command line, but apparently that
-- does not work.
-- Plugin setup.
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
	    "--log_dir=/home/fmil/tmp",
	    "--v=3",
	},
	root_dir = ".",
      })
    end
  }
)

require('pcc').setup()
