-- Spacing
vim.bo.tabstop = 4
vim.bo.shiftwidth = 4
vim.bo.expandtab = true

-- Format and organize imports on save
vim.api.nvim_create_autocmd("BufWritePre", {
	buffer = 0,
	callback = function()
		vim.lsp.buf.format { async = false }
		vim.lsp.buf.code_action {
			context = {
				only = { "source.organizeImports" },
				diagnostics = vim.diagnostic.get(0),
			},
			apply = true,
		}
	end,
})
