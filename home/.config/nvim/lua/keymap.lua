---@diagnostic disable: undefined-global

-- LINE
-- line wrap toggle
vim.keymap.set("", "<leader>lw", function()
	local wrap_enabled = vim.wo.wrap
	vim.wo.wrap = not wrap_enabled
	vim.wo.linebreak = not wrap_enabled
end, { desc = "line wrap toggle" })

-- line relative number toggle
vim.keymap.set("", "<leader>lr", function()
	vim.wo.relativenumber = not vim.wo.relativenumber
end, { desc = "relative numbers toggle" })

-- line number toggle
vim.keymap.set("", "<leader>ln", function()
	if vim.wo.number then
		if vim.wo.relativenumber then
			vim.wo.relativenumber = false
		end
	end
	vim.wo.number = not vim.wo.number
end, { desc = "line numbers toggle" })

-- diagnostic quickfix
vim.keymap.set("n", "<leader>q", vim.diagnostic.setloclist, { desc = "diagnostic quickfix list" })

-- exit terminal mode
vim.keymap.set("t", "<C-\\><C-n>", "<Esc><Esc>", { desc = "exit terminal mode" })
vim.keymap.set("t", "<Esc>", "<C-\\><C-n>", { desc = "exit terminal insert mode", noremap = true, silent = true })

-- clear highlights
vim.keymap.set("n", "<Esc>", "<cmd>nohlsearch<CR>")

-- resize windows
vim.keymap.set("n", "<C-w>=", "<cmd>wincmd =<CR>", { desc = "equalize window sizes" })
vim.keymap.set("n", "<C-w>H", "<cmd>vertical resize -12<CR>", { desc = "resize window right" })
vim.keymap.set("n", "<C-w>L", "<cmd>vertical resize +12<CR>", { desc = "resize window left" })
vim.keymap.set("n", "<C-w>J", "<cmd>resize +12<CR>", { desc = "resize window up" })
vim.keymap.set("n", "<C-w>K", "<cmd>resize -12<CR>", { desc = "resize window down" })

-- move lines up/down
vim.keymap.set("n", "<A-j>", ":m .+1<CR>==", { noremap = true, silent = true })
vim.keymap.set("n", "<A-k>", ":m .-2<CR>==", { noremap = true, silent = true })
vim.keymap.set("v", "<A-j>", ":m '>+1<CR>gv=gv", { noremap = true, silent = true })
vim.keymap.set("v", "<A-k>", ":m '<-2<CR>gv=gv", { noremap = true, silent = true })
