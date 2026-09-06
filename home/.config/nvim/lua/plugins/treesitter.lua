-- The `master` branch is EOL and only supports nvim <= 0.11. Its query
-- files call directives registered with the legacy `all = false` match
-- API, which nvim 0.12 removed -- every markdown fenced code block with
-- an info string then crashes the parser. `main` is the 0.12 branch.
local parsers = {
	"bash",
	"c",
	"css",
	"diff",
	"dockerfile",
	"git_config",
	"gitcommit",
	"gitignore",
	"go",
	"gomod",
	"gosum",
	"html",
	"javascript",
	"json",
	"lua",
	"luadoc",
	"markdown",
	"markdown_inline",
	"mermaid",
	"python",
	"query",
	"sql",
	"toml",
	"vim",
	"vimdoc",
	"yaml",
}

return {
	"nvim-treesitter/nvim-treesitter",
	branch = "main",
	build = ":TSUpdate",
	config = function()
		local ts = require("nvim-treesitter")

		-- Guard the bootstrap trap: if the checkout is still `master`,
		-- `install` is absent and erroring here takes startup down with
		-- it -- leaving no way in to run `:Lazy sync` and recover.
		if type(ts.install) ~= "function" then
			vim.notify(
				"nvim-treesitter: still on `master`; run :Lazy sync",
				vim.log.levels.WARN
			)
			return
		end

		ts.setup()
		ts.install(parsers)

		-- `main` ships no highlighting toggle of its own; nvim owns it.
		vim.api.nvim_create_autocmd("FileType", {
			group = vim.api.nvim_create_augroup("ts_start", { clear = true }),
			callback = function(ev)
				pcall(vim.treesitter.start, ev.buf)
			end,
		})

		vim.api.nvim_set_option_value("syntax", "off", { scope = "global" })
	end,
}
