return {
	"tpope/vim-fugitive",
	cmd = { "Git", "GBrowse" },
	keys = {
		{ "<leader>gB", "<cmd>Git blame<cr>", desc = "Blame file" },
	},
}
