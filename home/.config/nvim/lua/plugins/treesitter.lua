return {
	"nvim-treesitter/nvim-treesitter",
	build = ":TSUpdate",
	config = function()
		require("nvim-treesitter").setup({
			ensure_installed = { "go", "lua", "vim", "vimdoc" },
			auto_install = true,
		})
		vim.api.nvim_set_option_value("syntax", "off", { scope = "global" })
	end,
}
