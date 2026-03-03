return {
	"nvim-treesitter/nvim-treesitter",
	build = ":TSUpdate",
	config = function()
		require("nvim-treesitter.configs").setup({
			ensure_installed = { "go", "lua", "vim", "vimdoc" },
			auto_install = true,
			highlight = {
				enable = true,
			},
		})
	end,
}
