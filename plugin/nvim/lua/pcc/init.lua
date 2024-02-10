-- Try print something from vim.

P = function(v)
  print(vim.inspect(v))
  return v
end

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
        return ""
    end
    if r["err"] ~= nil then
        -- Log stuff here and below.
        return ""
    end
    if r["result"] == nil then
        return ""
    end
    if r["result"]["content"] == nil then
        return ""
    end
    return r["result"]["content"]
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
        file = string.format("file://%s", parent_buf_path),
        line = cursor_line,
        content = content,
    }, nil, parent_buf)
end

local M = {
    get_comment = get,
    set_comment = set,
}

function M.setup()
    -- Add something here.
end

function M.handlers()
    -- Apparently, these are handlers for messages that are sent from the lsp
    -- server to here.  While we don't use them, we must define them so that
    -- our requests from `get` and `set` would be honored. Why this works this
    -- way, I have no idea.
    return {
        ["$/pcc/set"] = function() end,
        ["$/pcc/get"] = function() end,
    }
end

return M

