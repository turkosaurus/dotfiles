vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.mouse = "a"
vim.opt.showmode = false -- Don't show the mode, since it's already in the status line
vim.opt.breakindent = true
vim.opt.undofile = true
vim.opt.ignorecase = true
vim.opt.smartcase = true
vim.opt.signcolumn = "yes"
vim.opt.updatetime = 250
vim.opt.timeoutlen = 300
vim.opt.splitright = true
vim.opt.splitbelow = true

-- cursor
vim.opt.cursorline = true

vim.opt.scrolloff = 10
vim.opt.confirm = true
vim.opt.tabstop = 4
vim.opt.shiftwidth = 4
vim.opt.expandtab = false
vim.opt.wrap = false
vim.opt.linebreak = true
vim.opt.breakindent = true

-- Fold options
vim.opt.foldmethod = "expr"
vim.opt.foldexpr = "nvim_treesitter#foldexpr()"
vim.opt.foldenable = false

-- Sets how neovim will display certain whitespace characters in the editor.
--  See `:help 'list'`
--  and `:help 'listchars'`
-- commenting out to fix errors with lines on go files per
-- https://github.com/nvim-lua/kickstart.nvim/issues/1237
-- vim.opt.list = true
--
-- vim.opt.listchars = { tab = "» ", trail = "·", nbsp = "␣" }

-- Preview substitutions live, as you type!
vim.opt.inccommand = "split"

-- Automatically start terminal in insert mode
vim.api.nvim_create_autocmd("TermOpen", {
    pattern = "*",
    command = "startinsert",
})

-- Highlight when yanking (copying) text
--  Try it with `yap` in normal mode
--  See `:help vim.highlight.on_yank()`
vim.api.nvim_create_autocmd("TextYankPost", {
    desc = "Highlight when yanking (copying) text",
    group = vim.api.nvim_create_augroup(
        "kickstart-highlight-yank",
        { clear = true }
    ),
    callback = function()
        vim.highlight.on_yank()
    end,
})

-- Show diagnostics on hover
vim.api.nvim_create_autocmd("CursorHold", {
    callback = function()
        vim.diagnostic.open_float(nil, { focusable = false })
    end,
})

-- Format and organize imports on save for Go files
vim.api.nvim_create_autocmd("BufWritePre", {
    pattern = "*.go",
    callback = function()
        vim.bo.tabstop = 4
        vim.bo.shiftwidth = 4
        vim.bo.expandtab = true
        -- vim.bo.softtabstop = 4
        vim.lsp.buf.format { async = false }
        vim.lsp.buf.code_action {
            context = {
                only = { "source.organizeImports" },
                diagnostics = vim.diagnostic.get(0),
            },
            apply = true,
        }
    end,
})

-- [[ Setup Alt line moving ]]
-- Move a line up or down in normal mode
vim.api.nvim_set_keymap("n", "<A-j>", ":m .+1<CR>==", {
    noremap = true,
    silent = true,
})
vim.api.nvim_set_keymap("n", "<A-k>", ":m .-2<CR>==", {
    noremap = true,
    silent = true,
})

-- Move selected lines up or down in visual mode
vim.api.nvim_set_keymap("v", "<A-j>", ":m '>+1<CR>gv=gv", {
    noremap = true,
    silent = true,
})
vim.api.nvim_set_keymap("v", "<A-k>", ":m '<-2<CR>gv=gv", {
    noremap = true,
    silent = true,
})

-- Spell Checker
vim.opt.spell = true
vim.opt.spelllang = "en"
vim.api.nvim_create_autocmd("FileType", {
    pattern = { "markdown", "text", "gitcommit" },
    callback = function()
        vim.opt_local.spell = true
        vim.opt_local.spelllang = "en"
    end,
})

-- Window separator
vim.opt.fillchars:append { vert = "┃" } -- Use a thicker vertical separator
vim.api.nvim_set_hl(
    0,
    "WinSeparator",
    { bold = false } -- Change color to green for better visibility
)
vim.opt.winhighlight = "VertSplit:WinSeparator"
