return {
	"folke/tokyonight.nvim",
	priority = 1000,
	config = function()
		require("tokyonight").setup({
			styles = {
				comments = { italic = false },
			},
			on_highlights = function(hl, c)
				-- the default gutter palette is too muted; these match
				-- the terminal's ANSI green/yellow/red under the
				-- TokyoNight Night ghostty theme
				hl.GitSignsAdd = { fg = c.green }
				hl.GitSignsChange = { fg = c.yellow }
				hl.GitSignsDelete = { fg = c.red }
			end,
		})
		vim.cmd.colorscheme("tokyonight-night")
	end,
}
