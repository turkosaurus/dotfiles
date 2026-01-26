--  NOTE: Must happen before plugins are loaded (otherwise wrong leader will be used)
vim.g.mapleader = " "
vim.g.maplocalleader = " "

if not vim.g.have_nerd_font then
    local handle = io.popen "fc-list : family | grep 'Nerd Font'"
    local result = handle and handle:read "*a" or ""
    if handle then
        handle:close()
    end
    if result ~= "" then
        vim.g.have_nerd_font = true
    else
        vim.g.have_nerd_font = false
    end
end

require "options"

-- Sync clipboard between OS and Neovim.
vim.schedule(function()
    vim.opt.clipboard = "unnamedplus"
end)

require "keymaps"
require "lazy-bootstrap"
require "lazy-plugins"
