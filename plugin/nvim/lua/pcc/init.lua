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
                message = "no pcc client was found",
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
    if r == nil then
        return {}
    end
    if r["err"] ~= nil then
        -- Log stuff here and below.
        return {}
    end
    if r["result"] == nil then
        return {}
    end
    if r["result"]["content"] == nil then
        return {}
    end
    return r["result"]["content"]
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

    if client == nil then
        local err_msg = string.format(
            "pcc: client: name=%s; bufnr=%d; line=%d; path=%s, bufs=%s",
                client_name, parent_buf, cursor_line, parent_buf_path, buffers)
        -- I bet this will be the mysterious error...
        -- I was right.
        return {
            err = {
                code = 44,
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
    pcc_binary = os.getenv("PCC_BINARY") or (vim.fn.stdpath("data") .. "/pcc/pcc"),

    -- Database could be in the local state directory by default.
    db = os.getenv("PCC_DB") or (vim.fn.stdpath("state") .. "/pcc/db/db.sqlite"),

    -- At startup, we will walk up the filesystem paths to find the workspace.
    -- If we find any of these files (or dirs, no matter), that's where we will
    -- consider the workspace to start.
    root_patterns = {
        ".git",
        -- From //nvim_testing/content:workspace.marker
        "workspace.marker",
    },

    file_patterns = { "text" },

    -- Set to something higher than 0 to have the "pcc" binary log verbose
    -- diagnostics.
    log_verbosity = 0,

    -- The default binding to the "add or edit" command.
    annotate_command = "<leader>cr",

    -- The default binding to the "delete" command.
    delete_command = "<leader>cd",
}

function M.setup(opts)
    M.config = vim.tbl_deep_extend('force', default_opts, opts or {})
    local client = find_client()
    vim.lsp.set_log_level("debug")
    vim.api.nvim_create_autocmd(
      { "FileType" },
      {
        pattern = M.config.file_patterns,
        nested = true,
        callback = function()
          if client == nil then
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
            handlers = require('pcc').handlers(),
          })
        end
      }
    )

    if client ~= nil then
        vim.keymap.set({'n'}, M.config.annotate_command,
            function()
                require('pcc').edit()
            end)
        vim.keymap.set({'n'}, M.config.delete_command,
            function()
                require('pcc').delete()
            end)
    end
end

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
    local annot_buf, annot_win = create_annot_buf(buf_info, annotation)
    -- Open buffer in a window, and pass buff info there.
end

-- Deletes the note at the current line of the current buffer.  Nothing happens
-- if there isn't a note there.
function M.delete()
    local buf_info = get_current_buf_info()
    set({}, buf_info)

end

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

return M

