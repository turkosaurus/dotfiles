return {
	{ "williamboman/mason.nvim", opts = {} },
	{ "j-hui/fidget.nvim", opts = {} },
	{
		"WhoIsSethDaniel/mason-tool-installer.nvim",
		dependencies = { "williamboman/mason.nvim" },
		config = function()
			require("mason-tool-installer").setup({
				ensure_installed = {
					"gopls",
					"pyright",
					"bash-language-server",
					"lua-language-server",
					"stylua",
					"shfmt",
					"shellcheck",
					"yaml-language-server",
				},
			})

			-- LspAttach keymaps (only Telescope overrides — grn, gra, grD are built-in)
			vim.api.nvim_create_autocmd("LspAttach", {
				group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
				callback = function(event)
					local map = function(keys, func, desc, mode)
						mode = mode or "n"
						vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "lsp: " .. desc })
					end

					map("grr", require("telescope.builtin").lsp_references, "references")
					map("gri", require("telescope.builtin").lsp_implementations, "implementations")
					map("grd", require("telescope.builtin").lsp_definitions, "definitions")
					map("gO", require("telescope.builtin").lsp_document_symbols, "document symbols")
					map("gW", require("telescope.builtin").lsp_dynamic_workspace_symbols, "workspace symbols")
					map("grt", require("telescope.builtin").lsp_type_definitions, "type definitions")

					local client = vim.lsp.get_client_by_id(event.data.client_id)

					-- highlight references
					if
						client
						and client:supports_method(vim.lsp.protocol.Methods.textDocument_documentHighlight, event.buf)
					then
						local highlight_augroup = vim.api.nvim_create_augroup("lsp-highlight", { clear = false })
						vim.api.nvim_create_autocmd({ "CursorHold", "CursorHoldI" }, {
							buffer = event.buf,
							group = highlight_augroup,
							callback = vim.lsp.buf.document_highlight,
						})
						vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI" }, {
							buffer = event.buf,
							group = highlight_augroup,
							callback = vim.lsp.buf.clear_references,
						})
						vim.api.nvim_create_autocmd("LspDetach", {
							group = vim.api.nvim_create_augroup("lsp-detach", { clear = true }),
							callback = function(event2)
								vim.lsp.buf.clear_references()
								vim.api.nvim_clear_autocmds({
									group = "lsp-highlight",
									buffer = event2.buf,
								})
							end,
						})
					end

					-- inlay hints toggle
					if
						client
						and client:supports_method(vim.lsp.protocol.Methods.textDocument_inlayHint, event.buf)
					then
						map("<leader>th", function()
							vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
						end, "toggle inlay hints")
					end
				end,
			})

			-- diagnostics
			vim.diagnostic.config({
				severity_sort = true,
				float = { border = "rounded", source = "if_many" },
				underline = { severity = vim.diagnostic.severity.ERROR },
				signs = vim.g.have_nerd_font and {
					text = {
						[vim.diagnostic.severity.ERROR] = "󰅚 ",
						[vim.diagnostic.severity.WARN] = "󰀪 ",
						[vim.diagnostic.severity.INFO] = "󰋽 ",
						[vim.diagnostic.severity.HINT] = "󰌶 ",
					},
				} or {},
				virtual_lines = true,
			})

			-- server configs (blink.cmp auto-applies capabilities via vim.lsp.config('*'))
			vim.lsp.config("gopls", {
				cmd = { "gopls", "serve" },
				filetypes = { "go", "gomod" },
				settings = {
					gopls = {
						buildFlags = { "-tags=integration" },
						gofumpt = true,
						staticcheck = true,
						hoverKind = "FullDocumentation",
						usePlaceholders = false,
						completeUnimported = true,
						symbolMatcher = "FastFuzzy",
						semanticTokens = true,
						experimentalPostfixCompletions = true,
						directoryFilters = { "-vendor", "-.git" },
						codelenses = {
							gc_details = true,
						},
						analyses = {
							nilness = true,
							unusedparams = true,
							unusedwrite = true,
							shadow = true,
						},
						hints = {
							parameterNames = true,
							rangeVariableTypes = true,
							compositeLiteralFields = true,
							compositeLiteralTypes = true,
							constantValues = true,
							functionTypeParameters = true,
						},
					},
				},
			})

			vim.lsp.config("pyright", {})

			vim.lsp.config("bashls", {
				cmd = { "bash-language-server", "start" },
				filetypes = { "sh", "zsh", "bash" },
				settings = {
					bashIde = {
						shellcheckPath = "", -- disable shellcheck (workaround for unicode parse bug)
					},
				},
			})

			vim.lsp.config("lua_ls", {
				settings = {
					Lua = {
						completion = {
							callSnippet = "Replace",
						},
					},
				},
			})

			vim.lsp.config("yamlls", {
				settings = {
					yaml = {
						schemaStore = {
							enable = true,
							url = "https://www.schemastore.org/api/json/catalog.json",
						},
						schemas = {
							["https://json.schemastore.org/github-workflow.json"] = "/.github/workflows/*",
						},
					},
				},
			})

			vim.lsp.enable({ "gopls", "pyright", "bashls", "lua_ls", "yamlls" })
		end,
	},
}
