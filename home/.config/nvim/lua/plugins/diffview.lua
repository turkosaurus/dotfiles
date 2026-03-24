return {
	"sindrets/diffview.nvim",
	cmd = { "DiffviewOpen", "DiffviewFileHistory" },
	keys = {
		{ "<leader>gd", "<cmd>DiffviewOpen<cr>", desc = "Diff vs index" },
		{ "<leader>gD", "<cmd>DiffviewOpen main<cr>", desc = "Diff vs main" },
		{ "<leader>gh", "<cmd>DiffviewFileHistory %<cr>", desc = "File history" },
		{ "<leader>gq", "<cmd>DiffviewClose<cr>", desc = "Close diff" },
	},
	opts = {
		enhanced_diff_hl = true,
	},
}
