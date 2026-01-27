return {
	"nvim-neo-tree/neo-tree.nvim",
	version = "*",
	dependencies = {
		"nvim-lua/plenary.nvim",
		"nvim-tree/nvim-web-devicons",
		"MunifTanjim/nui.nvim",
	},
	cmd = "Neotree",
	keys = {
		{ "\\", ":Neotree reveal<CR>", desc = "neotree reveal", silent = true },
		{ "|", ":Neotree document_symbols<CR>", desc = "neotree symbols", silent = true },
	},
	opts = {
		sources = { "filesystem", "document_symbols" },
		source_selector = {
			winbar = true,
			statusline = false,
			sources = {
				{ source = "filesystem", display_name = " Files" },
				{ source = "document_symbols", display_name = "󰊕 Symbols" },
			},
		},
		filesystem = {
			follow_cursor = true,
			filtered_items = {
				hide_dotfiles = false,
				hide_gitignored = false,
			},
			window = {
				position = "right",
				width = 24,
				auto_expand_width = true,
				mappings = {
					["\\"] = "close_window",
				},
			},
		},
		document_symbols = {
			follow_cursor = true,
			kinds = {
				Class = { icon = "󰠱", hl = "TSClass" },
				Function = { icon = "󰊕", hl = "TSFunction" },
				Variable = { icon = "󰀫", hl = "TSVariable" },
			},
			window = {
				position = "right",
				width = 24,
				auto_expand_width = true,
				mappings = {
					["|"] = "close_window",
				},
			},
		},
		default_component_configs = {
			icon = {
				folder_closed = "[+]",
				folder_open = "[-]",
				folder_empty = "[ ]",
				default = "-",
			},
		},
		event_handlers = {
			{
				event = "file_opened",
				handler = function()
					require("neo-tree.command").execute({ action = "close" })
				end,
			},
		},
	},
}
