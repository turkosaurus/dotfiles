-- LINE --
-- Line wrap toggle
vim.keymap.set("", "<leader>lw", function()
    local wrap_enabled = vim.wo.wrap
    vim.wo.wrap = not wrap_enabled
    vim.wo.linebreak = not wrap_enabled
end, { desc = "[L]ine [W]rap toggle" })

-- Line relative number shortcut
vim.keymap.set("", "<leader>lr", function()
    vim.wo.relativenumber = not vim.wo.relativenumber
end, { desc = "[L]ine [R]elative Numbers toggle" })

-- Line number toggle
vim.keymap.set("", "<leader>ln", function()
    if vim.wo.number then
        if vim.wo.relativenumber then
            vim.wo.relativenumber = false
        end
    end
    vim.wo.number = not vim.wo.number
end, { desc = "[L]ine [N]umbers toggle" })

-- Diagnostic keymaps
vim.keymap.set(
    "n",
    "<leader>q",
    vim.diagnostic.setloclist,
    { desc = "Open diagnostic [Q]uickfix list" }
)

-- Exit terminal mode in the builtin terminal with a shortcut that is a bit easier
-- for people to discover. Otherwise, you normally need to press <C-\><C-n>, which
-- is not what someone will guess without a bit more experience.
-- NOTE: This won't work in all terminal emulators/tmux/etc. Try your own mapping
-- or just use <C-\><C-n> to exit terminal mode
vim.keymap.set(
    "t",
    "<C-\\><C-n>",
    "<Esc><Esc>",
    { desc = "Exit terminal mode" }
)

vim.keymap.set("t", "<Esc>", "<C-\\><C-n>", {
    desc = "Exit terminal insert mode",
    noremap = true,
    silent = true,
})

-- Clear highlights on search when pressing <Esc> in normal mode
--  See `:help hlsearch`
vim.keymap.set("n", "<Esc>", "<cmd>nohlsearch<CR>")

-- Resize just like .tmux.conf
vim.keymap.set(
    "n",
    "<C-w>H",
    "<cmd>vertical resize +24<CR>",
    { desc = "Resize window right by 24" }
)
vim.keymap.set(
    "n",
    "<C-w>L",
    "<cmd>vertical resize -24<CR>",
    { desc = "Resize window left by 24" }
)
vim.keymap.set(
    "n",
    "<C-w>J",
    "<cmd>resize -24<CR>",
    { desc = "Resize window up by 24" }
)
vim.keymap.set(
    "n",
    "<C-w>K",
    "<cmd>resize +24<CR>",
    { desc = "Resize window down by 24" }
)
