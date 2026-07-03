return {
	"stevearc/conform.nvim",
	event = { "BufWritePre", "BufReadPost" },
	cmd = { "ConformInfo" },
	keys = {
		{
			"<leader>f",
			function()
				require("conform").format({ async = true, lsp_format = "fallback" })
			end,
			mode = "",
			desc = "format buffer",
		},
	},
	opts = {
		notify_on_error = false,
		format_on_save = function(bufnr)
			local disable_filetypes = { c = true, cpp = true, markdown = true }
			if disable_filetypes[vim.bo[bufnr].filetype] then
				return nil
			else
				return { timeout_ms = 500, lsp_format = "fallback" }
			end
		end,
		formatters_by_ft = {
			lua = { "stylua" },
			sh = { "shfmt" },
			bash = { "shfmt" },
			zsh = { "shfmt" },
			toml = { "taplo" },
		},
		formatters = {
			shfmt = {
				prepend_args = { "-i", "2", "-ci" },
			},
			-- taplo's config auto-search only walks up from cwd; it never
			-- looks in ~/.config/taplo/. Pass -c explicitly so global
			-- defaults apply even when no local .taplo.toml exists.
			taplo = {
				prepend_args = { "-c", vim.fn.expand("~/.config/taplo/taplo.toml") },
			},
		},
	},
	config = function(_, opts)
		require("conform").setup(opts)

		-- Format-on-open for filetypes with an entry in formatters_by_ft.
		-- Pairs with format_on_save so the buffer is canonical when a file
		-- is opened *and* when it's written.
		vim.api.nvim_create_autocmd("BufReadPost", {
			group = vim.api.nvim_create_augroup("conform-format-on-open", { clear = true }),
			callback = function(args)
				if not vim.bo[args.buf].modifiable or vim.bo[args.buf].readonly then
					return
				end
				local ft = vim.bo[args.buf].filetype
				if opts.formatters_by_ft[ft] == nil then
					return
				end
				require("conform").format({ bufnr = args.buf, timeout_ms = 500, lsp_format = "fallback" })
			end,
		})
	end,
}
