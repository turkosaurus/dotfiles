return {
	"mfussenegger/nvim-lint",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local lint = require("lint")
		lint.linters_by_ft = {
			sh = { "shellcheck" },
			bash = { "shellcheck" },
			go = { "golangcilint" },
		}
		vim.api.nvim_create_autocmd({ "BufEnter", "BufWritePost", "InsertLeave" }, {
			callback = function()
				if vim.bo.filetype == "go" then
					local mod = vim.fs.find("go.mod", {
						upward = true,
						path = vim.fn.expand("%:p:h"),
					})
					if #mod == 0 then return end
				end
				lint.try_lint()
			end,
		})
	end,
}
