P = function(thing)
  print(vim.inspect(thing))
  return thing
end

vim.g.mapleader = ','
vim.g.maplocalleader='\\'
vim.lsp.set_log_level("debug")

-- From //:marker
local root_patterns = { ".pcc.config.json" }

local db_name = os.getenv("PCC_DB")

if db_name == nil or db_name == "" then
  db_name = "file:test.db?cache=shared&mode=memory"
end

require('pcc').setup({
  db = db_name,
  root_patterns = root_patterns,
  log_verbosity = 3,
})

