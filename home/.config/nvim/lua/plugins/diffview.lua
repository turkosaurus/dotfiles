return {
	"sindrets/diffview.nvim",
	cmd = { "DiffviewOpen", "DiffviewFileHistory" },
	keys = {
		{ "<leader>gd", "<cmd>DiffviewOpen<cr>", desc = "Diff vs index" },
		{ "<leader>gD", "<cmd>DiffviewOpen main<cr>", desc = "Diff vs main" },
		{ "<leader>gh", "<cmd>DiffviewFileHistory %<cr>", desc = "File history" },
		{ "<leader>gq", "<cmd>DiffviewClose<cr>", desc = "Close diff" },
	},
	config = function()
		local actions = require("diffview.actions")
		require("diffview").setup({
			enhanced_diff_hl = true,
			keymaps = {
				file_panel = {
					{ "n", "<cr>", function()
						actions.select_entry()
						vim.defer_fn(function()
							local buf = vim.api.nvim_get_current_buf()
							if vim.bo[buf].filetype == "DiffviewFiles" then
								vim.cmd("wincmd l | wincmd l")
							end
						end, 300)
					end, { desc = "Open and focus diff" } },
				},
			},
		})
	end,
}
