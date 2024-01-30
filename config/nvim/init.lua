-- would have been better if I could add from a command line, but apparently that
-- does not work.
vim.opt.rtp:prepend("./plugin")
vim.print(vim.api.nvim_list_runtime_paths(), "\n")

require('pcc')
