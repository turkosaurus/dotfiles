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
		notify_on_error = true,
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
			toml = { "taplo", "plan_toml_unfurl" },
		},
		formatters = {
			shfmt = {
				prepend_args = { "-i", "2", "-ci" },
			},
			-- taplo's config auto-search only walks up from cwd; it never
			-- looks in ~/.config/taplo/. Pass -c explicitly so global
			-- defaults apply even when no local .taplo.toml exists.
			-- Override args (not prepend_args) because -c is a `format`
			-- subcommand flag — prepending would place it before `format`
			-- and taplo would reject it as an unknown root option.
			taplo = {
				args = {
					"format",
					"-c",
					vim.fn.expand("~/.config/taplo/taplo.toml"),
					"--stdin-filepath",
					"$FILENAME",
					"-",
				},
			},
			-- Post-process only plan-shaped TOML: rewrite "..."/\n strings
			-- emitted by yq into '''...''' multi-line literals per the
			-- plan-file style guide. Scoped to files under ~/w/ (worktree
			-- plans at ~/w/<repo>/<branch>/plan.toml and task plans at
			-- ~/w/t/<status>/<N>.toml) so ordinary TOML never runs it.
			plan_toml_unfurl = {
				command = vim.fn.expand("~/bin/plan-toml-unfurl"),
				stdin = true,
				condition = function(_, ctx)
					local prefix = vim.fn.expand("~/w/")
					local path = ctx.filename or ""
					return path:sub(1, #prefix) == prefix
				end,
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
