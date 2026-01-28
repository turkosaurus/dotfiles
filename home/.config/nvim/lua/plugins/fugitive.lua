return {
	"tpope/vim-fugitive",
	keys = {
		-- Git status and blame
		{ "<leader>gs", "<cmd>Git<cr>", desc = "Git status" },
		{ "<leader>gB", "<cmd>Git blame<cr>", desc = "Blame file" },

		-- Merge conflict resolution
		{ "<leader>gm", "<cmd>Gvdiffsplit!<cr>", desc = "Open 3-way merge" },
		{ "<leader>gh", "<cmd>diffget //2<cr>", desc = "Get from left (target)" },
		{ "<leader>gl", "<cmd>diffget //3<cr>", desc = "Get from right (merge)" },
		{ "<leader>gw", "<cmd>Gwrite<cr>", desc = "Stage resolved file" },

		-- Navigate conflicts
		{ "]c", desc = "Next conflict" },
		{ "[c", desc = "Previous conflict" },

		-- Pull Request diff view
		{ "<leader>gd", "<cmd>Gvdiffsplit<cr>", desc = "Open PR diff view" },
	},
}
