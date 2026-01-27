require("lazy").setup({
	change_detection = { notify = false },
	spec = {
		{ "nvim-lua/plenary.nvim", lazy = true },
		{ "tpope/vim-sleuth" },
		{ import = "plugins" },
	},
})
