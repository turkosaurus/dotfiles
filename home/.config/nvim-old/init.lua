--[[
=====================================================================
=====================================================================
========                                    .-----.          ========
========         .----------------------.   | === |          ========
========         |.-""""""""""""""""""-.|   |-----|          ========
========         ||                    ||   | === |          ========
========         ||   KICKSTART.NVIM   ||   |-----|          ========
========         ||                    ||   | === |          ========
========         ||                    ||   |-----|          ========
========         ||:Tutor              ||   |:::::|          ========
========         |'-..................-'|   |____o|          ========
========         `"")----------------(""`   ___________      ========
========        /::::::::::|  |::::::::::\  \ no mouse \     ========
========       /:::========|  |==hjkl==:::\  \ required \    ========
========      '""""""""""""'  '""""""""""""'  '""""""""""'   ========
========                                                     ========
=====================================================================
=====================================================================
-- ]]

-- Set <space> as the leader key
-- See `:help mapleader`
--  NOTE: Must happen before plugins are loaded (otherwise wrong leader will be used)
vim.g.mapleader = " "
vim.g.maplocalleader = " "

-- Set to true if you have a Nerd Font installed and selected in the terminal
-- TODO: check this dynamically for portability
vim.g.have_nerd_font = true

-- [[ Setting options ]]
-- See `:help vim.opt`
--  For more options, you can see `:help option-list`
require "options"

-- Sync clipboard between OS and Neovim.
--  Schedule the setting after `UiEnter` because it can increase startup-time.
--  Remove this option if you want your OS clipboard to remain independent.
--  See `:help 'clipboard'`
vim.schedule(function()
    vim.opt.clipboard = "unnamedplus"
end)

-- [[ Basic Keymaps ]]
--  See `:help vim.keymap.set()`
require "keymaps"

-- [[ Basic Autocommands ]]
--  See `:help lua-guide-autocommands`

-- [[ Install `lazy.nvim` plugin manager ]]
require "lazy-bootstrap"

-- [[ Configure and install plugins ]]
--  To check the current status of your plugins, run
--    :Lazy
--
--  You can press `?` in this menu for help. Use `:q` to close the window
--
--  To update plugins you can run
--    :Lazy update
--
require "lazy-plugins"

-- TODO: find a support for this?
-- vim.api.nvim_set_keymap(
--     "",
--     "<CapsLock>",
--     "<Esc>",
--     { noremap = true, silent = true }
-- )
-- vim.api.nvim_set_keymap(
--     "i",
--     "<CapsLock>",
--     "<Esc>",
--     { noremap = true, silent = true }
-- )
-- vim.api.nvim_set_keymap(
--     "v",
--     "<CapsLock>",
--     "<Esc>",
--     { noremap = true, silent = true }
-- )
