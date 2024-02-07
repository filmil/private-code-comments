-- Try print something from vim.

local client_name = 'pcc'

local method_get = '$/pcc/get' -- file, line -> text or ""
local method_set = '$/pcc/set' -- file, line, text -> (nothing)

local function get()
    local parent_win = vim.api.nvim_get_current_win()
    local parent_buf = vim.api.nvim_win_get_buf(parent_win)
    local parent_buf_path = vim.api.nvim_buf_get_name(parent_buf)
    local cursor_line = vim.api.nvim_win_get_cursor(parent_win)[1] - 1

    local client = vim.lsp.get_active_clients({
        name = client_name,
        bufnr = parent_buf,
    })[1]
    if client == nil then
        -- I bet this will be the mysterious error...
        return "e:1"
    end
    return client.request_sync(method_get, {
        file = parent_buf_path, -- probably needs to be formatted as URI.
        line = cursor_line,
    }, 100, parent_buf)
end

local function set(content)
    local parent_win = vim.api.nvim_get_current_win()
    local parent_buf = vim.api.nvim_win_get_buf(parent_win)
    local parent_buf_path = vim.api.nvim_buf_get_name(parent_buf)
    local cursor_line = vim.api.nvim_win_get_cursor(parent_win)[1] - 1

    local buffers = vim.api.nvim_list_bufs()[1]

    local client = vim.lsp.get_active_clients({
        --name = client_name,
        bufnr = parent_buf,
    })[1] -- is this correct?

    if client == nil then
        -- I bet this will be the mysterious error...
        -- I was right.
        return string.format(
            "clientooga: name=%s; bufnr=%d; line=%d; path=%s, bufs=%s",
                client_name, parent_buf, cursor_line, parent_buf_path, buffers)
    end
    return client.request(method_set, {
        file = parent_buf_path,
        line = cursor_line,
        text = content,
    }, nil, parent_buf)
end

local M = {
    get_comment = set,
    --get_comment = function ()
        --return "oogabooga"
    --end,
    set_comment = function()
        return "oogabooga2"
    end,
}

function M.setup()
    -- Add something here.
end


return M

