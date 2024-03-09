local function is_win()
  return package.config:sub(1, 1) == '\\'
end

local function get_path_separator()
  if is_win() then
    return '\\'
  end
  return '/'
end


---Returns the path of this script.
---@return (string)
local function script_path()
  local str = debug.getinfo(2, 'S').source:sub(2)
  if is_win() then
    str = str:gsub('/', '\\')
  end
  return str:match('(.*' .. get_path_separator() .. ')')
end

---Docs here.

local function script_path()
  local str = debug.getinfo(2, 'S').source:sub(2)
  return str:match('(.*' .. get_path_separator() .. ')')
end

P = function(thing)
  print(vim.inspect(thing))
  return thing
end

local client_name = 'pcc'
local method_get = '$/pcc/get' -- file, line -> text or ""
local method_set = '$/pcc/set' -- file, line, text -> (nothing)

-- Returns the current buffer information.
local function get_current_buf_info()
    local parent_win = vim.api.nvim_get_current_win()
    local parent_buf = vim.api.nvim_win_get_buf(parent_win)
    local parent_buf_path = vim.api.nvim_buf_get_name(parent_buf)
    local cursor_line = vim.api.nvim_win_get_cursor(parent_win)[1] - 1

    return {
        parent_buf = parent_buf,
        parent_buf_path = parent_buf_path,
        cursor_line = cursor_line,
    }
end

-- Returns the pcc client for the given buffer (or nil) if it exists, or
-- nil if it does not.
local function find_client(bufnr)
    local opts = {
        name = client_name,
        bufnr = bufnr,
    }
    if bufnr == nil then
        opts = {
            name = client_name,
        }
    end
    local client = vim.lsp.get_active_clients(opts)[1]
    return client
end

local function get(buf_info)
    if buf_info == nil then
        buf_info = get_current_buf_info()
    end
    local parent_buf = buf_info.parent_buf
    local parent_buf_path = buf_info.parent_buf_path
    local cursor_line = buf_info.cursor_line

    local client = find_client(parent_buf)
    if client == nil then
        return {
            err = {
                code = 43,
                message = string.format(
                    "no pcc client was found: buf=%d - did it even start?", parent_buf),
            }
        }
    end

    -- The return values here are a mess. Yo uwill get `nil` on some errors;
    -- OR you get a result object which is a table like:
    -- {
    --   result = {
    --     -- your result
    --   }
    -- }
    -- OR you will get an error like:
    -- {
    --   err = {
    --     code = 42,
    --     message = "foo",
    --     data = <literally anything, including nil>
    --   }
    -- }
    --
    -- I am not a fan.
    local r = client.request_sync(method_get, {
        file = string.format("file://%s", parent_buf_path),
        line = cursor_line,
    }, 100, parent_buf)
    if not r then
        return {
            err = {
                code = 45,
                message = "no response received at all",
            }
        }
    end
    if r.err then
        return {
            err = {
                code = 47,
                message = "RPC returned an error - see 'data' for details",
                data = r,
            }
        }
    end
    if not r.result then
        return {
            err = {
                code = 46,
                message = "no result and no error either - how?",
            }
        }
    end
    return r.result.content or {
        err = {
            code = 44,
            message = "no content found - this is not supposed to happen",
        }
    }
end

local function set(content, buf_info)
    if buf_info == nil then
        buf_info = get_current_buf_info()
    end
    local parent_buf = buf_info.parent_buf
    local parent_buf_path = buf_info.parent_buf_path
    local cursor_line = buf_info.cursor_line

    local buffers = vim.api.nvim_list_bufs()[1]

    local client = vim.lsp.get_active_clients({
        name = client_name,
        bufnr = parent_buf,
    })[1] -- is this correct?

    if not client then
        local err_msg = string.format(
            "pcc: client: name=%s; bufnr=%d; line=%d; path=%s, bufs=%s",
                client_name, parent_buf, cursor_line, parent_buf_path, buffers)
        -- I bet this will be the mysterious error...
        -- I was right.
        return {
            err = {
                code = 48,
                message = err_msg,
            }
        }
    end
    return client.request(method_set, {
        file = string.format("file://%s", parent_buf_path),
        line = cursor_line,
        content = content,
    }, nil, parent_buf)
end

local M = {
    get_comment = get,
    set_comment = set,
}

local function create_annot_win(annot_buf, cursor_ln, extmark_parent_win, win_width, padding)
    local annot_win = vim.api.nvim_open_win(annot_buf, true, {
        relative = 'win',
        win = extmark_parent_win,
        anchor = 'NE',
        row = cursor_ln - 1,
        col = win_width - padding,
        width = M.config.annot_win_width,
        height = 10,
        border = 'rounded',
        style = 'minimal',
        title = 'Annotation',
        title_pos = 'center'
    })
    return annot_win
end

local default_opts = {
    -- This is how wide the annotation window will be.
    annot_win_width = 25,

    -- This is the padding, obviously.
    annot_win_padding = 2,

    -- This is the directory where the LSP client will write its logs.
    log_dir = os.getenv("PCC_LOG_DIR") or (vim.fn.stdpath("state") .. "/pcc/logs"),

    -- This is where to find the PCC binary.
    pcc_binary = os.getenv("PCC_BINARY") or (script_path() .. "/bin/pcc"),

    -- Database could be in the local state directory by default.
    db = os.getenv("PCC_DB") or (vim.fn.stdpath("state") .. "/pcc/db/db.sqlite"),

    -- At startup, we will walk up the filesystem paths to find the workspace.
    -- If we find any of these files (or dirs, no matter), that's where we will
    -- consider the workspace to start.
    root_patterns = {
        ".git",
        "pcc.config.json",
    },

    file_patterns = { "text" },

    filetypes = { "text", "lua", "rust", "gn" },

    -- Set to something higher than 0 to have the "pcc" binary log verbose
    -- diagnostics.
    log_verbosity = 0,

    autostart = true,
}

---Configures the pcc client side, without using lsp-config.
function M.setup_client(opts)
    M.config = vim.tbl_deep_extend('force', default_opts, opts or {})
    local client = find_client()
    vim.lsp.set_log_level("debug")
    vim.api.nvim_create_autocmd(
      { "FileType" },
      {
        pattern = M.config.file_patterns,
        nested = true,
        callback = function()
          if client ~= nil then
            return
          end
          vim.lsp.start({
            cmd = {
                M.config.pcc_binary,
                "--log_dir=" .. M.config.log_dir,
                "--v=" .. string.format("%d", M.config.log_verbosity),
                "--db=" .. M.config.db,
            },
            root_dir = vim.fs.dirname(
              vim.fs.find(M.config.root_patterns,
              { upward = true })[1]),
            handlers = M.handlers(),
          })
        end
      }
    )
end

---@deprecated
function M.setup(opts)
    M.setup_client(opts)
end

---Returns the numeric ID of the buffer with the given name.
---@param name (string)
---@return (integer|-1)
local function find_buf_by_name(name)
    local bufs = vim.api.nvim_list_bufs()
    for _, buf_id in ipairs(bufs) do
        local buf_name = vim.api.nvim_buf_get_name(buf_id)
        print(buf_name)
        if buf_name == name then
            return buf_id
        end
    end
    return -1
end

local function create_annot_buf(buf_info, annotation)
    local annot_buf_name = 'Annotation'
    local annot_buf = -1
    local extmark_parent_win = vim.api.nvim_get_current_win()
    local win_width = vim.api.nvim_win_get_width(extmark_parent_win)
    local padding = M.config.annot_win_padding
    local annot_win

    if annot_buf == -1 then
        annot_buf = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_lines(annot_buf, 0, -1, true, annotation)
        vim.api.nvim_buf_set_keymap(annot_buf, 'n', 'q', ':close<CR>',
            {noremap=true, silent=true, nowait=true})
        vim.schedule(function()
            -- Try to avoid "can not change name".
            --vim.api.nvim_buf_set_name(annot_buf, annot_buf_name)
        end)
    end

    local edit_group = vim.api.nvim_create_augroup('EditComment', {clear=true})

    -- When window is closed, save the annotation.
    vim.api.nvim_create_autocmd('BufHidden', {
        callback = function()
            local lines = vim.api.nvim_buf_get_lines(annot_buf, 0, -1, true)
            set(lines, buf_info)
        end,
        group = edit_group,
        buffer = annot_buf,
    })

    annot_win = create_annot_win(
        annot_buf, buf_info.cursor_line, extmark_parent_win, win_width, padding)

    return annot_buf, annot_win
end

-- Edits the note at the current line of the current buffer, or creates one
-- if it does not exist.
function M.edit()
    local buf_info = get_current_buf_info()

    local annotation = get(buf_info)
    if annotation.err and not annotation.msg then
        local e = annotation.err
        error(string.format("E%d: %s", e.code, e.message))
        return
    end

    local annot_buf, annot_win = create_annot_buf(buf_info, annotation)
    -- Open buffer in a window, and pass buf info there.
end

-- Deletes the note at the current line of the current buffer.  Nothing happens
-- if there isn't a note there.
function M.delete()
    local buf_info = get_current_buf_info()
    set({}, buf_info)

end

---Returns the handler table for the custom methods. These are unused, but
---must be defined so that we can issue these calls to the server.
function M.handlers()
    -- Apparently, these are handlers for messages that are sent from the lsp
    -- server to here.  While we don't use them, we must define them so that
    -- our requests from `get` and `set` would be honored. Why this works this
    -- way, I have no idea.
    return {
        [method_get] = function() end,
        [method_set] = function() end,
    }
end

---Setup when using with lsp-config plugin.
---
---@param opts (table|nil)  LSP-config compatible table, default used if nil.
---@param client_opts (table|nil)  Client config compatible table
function M.setup_server_with_lsp_config(opts, client_opts)
    M.config = vim.tbl_deep_extend('force', default_opts, client_opts or {})
    local lspconfig = require 'lspconfig'
    local configs = require 'lspconfig.configs'
    local cfg = vim.tbl_deep_extend('force',
        {
            name = "pcc",
            cmd = {
                M.config.pcc_binary,
                '--log_dir=' .. M.config.log_dir,
                '--v=' .. M.config.log_verbosity,
                '--db=' .. M.config.db,
            },
            root_dir = lspconfig.util.root_pattern(M.config.root_patterns),
            filetypes = M.config.filetypes,
            handlers = M.handlers(),
            autostart = M.autostart,
        },
        opts or {}
    )
    configs.pcc = configs.pcc or {
        default_config = cfg,
        docs = {
            description = [[
https://github.com/filmil/private-code-comments

## Installation

Download the [release archive][rel], and unpack it into the directory
`$HOME/.config`. This should create a directory `$HOME/.config/pcc` on your
disk.

[rel]: https://github.com/filmil/private-code-comments/releases

Add the following configuration in your `init.lua`.

```lua
-- This setup places the private comments plugin into the directory $HOME/.config/pcc.
require('lspconfig')
local pcc_dir = os.getenv("HOME") .. "/.config/pcc"
vim.opt.rtp:prepend(pcc_dir)
local pcc_plugin = require('pcc')

pcc_plugin.setup_server_with_lsp_config(
 {}, -- lspconfig settings
 {
     pcc_binary = pcc_dir .. "/bin/pcc",
     log_dir = os.getenv("HOME") .. '/.local/state/pcc/logs',
     db = os.getenv("HOME") .. '/.local/state/pcc/db/db.sqlite',
     log_verbosity = 3,
 }
)
require('lspconfig').pcc.setup {}

vim.keymap.set({'n'}, '<leader>cr', pcc_plugin.edit, { desc = "[C]omment [R]eview" })
vim.keymap.set({'n'}, '<leader>cd', pcc_plugin.delete, { desc = "[C]omment [D]elete" })
```
        ]],
        },
    }
end

return M

