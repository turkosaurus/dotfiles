-- Spacing
vim.bo.tabstop = 4
vim.bo.shiftwidth = 4
vim.bo.expandtab = true

-- Format and organize imports on save.
-- We do the code-action request directly rather than calling
-- vim.lsp.buf.code_action({apply=true}), which shows a
-- "No code actions available" notification when gopls has nothing to do.
vim.api.nvim_create_autocmd("BufWritePre", {
	buffer = 0,
	callback = function()
		local params = vim.lsp.util.make_range_params(0, "utf-8")
		params.context = { only = { "source.organizeImports" }, diagnostics = {} }
		local result = vim.lsp.buf_request_sync(0, "textDocument/codeAction", params, 1000)
		for cid, res in pairs(result or {}) do
			for _, r in ipairs(res.result or {}) do
				if r.edit then
					local enc = (vim.lsp.get_client_by_id(cid) or {}).offset_encoding or "utf-16"
					vim.lsp.util.apply_workspace_edit(r.edit, enc)
				end
			end
		end
		vim.lsp.buf.format { async = false }
	end,
})
