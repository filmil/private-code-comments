P = function(thing)
  print(vim.inspect(thing))
  return thing
end

vim.g.mapleader = ','
vim.g.maplocalleader='\\'
vim.lsp.set_log_level("debug")

-- From //:marker
local root_patterns = { "pcc.config.json" }
local db_name = os.getenv("PCC_DB")
if db_name == nil or db_name == "" then
  db_name = "file:test.db?cache=shared&mode=memory"
end

-- PCC client setup here.
local pcc_client = require('pcc')
pcc_client.setup_client({
  db = db_name,
  root_patterns = root_patterns,
  log_verbosity = 3,
})
vim.keymap.set({'n'}, "<leader>cr", pcc_client.edit)
vim.keymap.set({'n'}, "<leader>cd", pcc_client.delete)

