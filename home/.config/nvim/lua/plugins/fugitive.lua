return {
	"tpope/vim-fugitive",
	cmd = {
		"Git",
		"Gwrite",
		"Gread",
		"Gedit",
		"Gdiffsplit",
		"Gvdiffsplit",
		"GMove",
		"GDelete",
		"GBrowse",
	},
	keys = {
		{ "<leader>gs", "<cmd>Git<cr>", desc = "Git status" },
		{ "<leader>gB", "<cmd>Git blame<cr>", desc = "Blame file" },

		-- 3-way merge
		{ "<leader>gm", "<cmd>Gvdiffsplit!<cr>", desc = "3-way merge" },
		{ "<leader>gh", "<cmd>diffget //2<cr>", desc = "Get hunk from left" },
		{ "<leader>gl", "<cmd>diffget //3<cr>", desc = "Get hunk from right" },
		{ "<leader>g2", "<cmd>Gread :2<cr>", desc = "Accept entire left" },
		{ "<leader>g3", "<cmd>Gread :3<cr>", desc = "Accept entire right" },
		{ "<leader>gw", "<cmd>Gwrite<cr>", desc = "Stage file" },
		{ "<leader>gq", "<cmd>diffoff | only<cr>", desc = "Close diff" },
		{ "<leader>dp", "dp", desc = "Put diff to other" },

		{ "<leader>gd", "<cmd>Gvdiffsplit<cr>", desc = "Diff split" },
	},
}
