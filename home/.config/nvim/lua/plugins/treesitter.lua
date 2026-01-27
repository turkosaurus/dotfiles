return {
	"nvim-treesitter/nvim-treesitter",
	build = ":TSUpdate",
	opts = {},
	config = function(_, opts)
		require("nvim-treesitter.config").setup(vim.tbl_extend("force", {
			ensure_installed = { "lua", "vim", "vimdoc" },
			auto_install = true,
			highlight = { enable = true },
			indent = { enable = true },
			fold = { enable = true },
		}, opts))
	end,
}
