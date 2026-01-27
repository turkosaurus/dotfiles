local has_termcolor, _ = pcall(require, "termcolor")

local signs
if has_termcolor then
	signs = {
		add = { text = "+" },
		change = { text = "~" },
		delete = { text = "_" },
		topdelete = { text = "‾" },
		changedelete = { text = "~" },
	}
else
	signs = {
		add = { text = "|" },
		change = { text = "|" },
		delete = { text = "|" },
		topdelete = { text = "|" },
		changedelete = { text = "|" },
	}
end

return {
	"lewis6991/gitsigns.nvim",
	keys = {
		{ "<leader>gb", "<cmd>Gitsigns toggle_current_line_blame<cr>", desc = "Toggle git blame line" },
	},
	opts = {
		signs = signs,
		current_line_blame = true,
		current_line_blame_formatter = "[<abbrev_sha>] <summary> • <author>, <author_time:%R> ",
		current_line_blame_opts = {
			virt_text = true,
			virt_text_pos = "right_align", -- 'eol' | 'overlay' | 'right_align'
			delay = 800,
			ignore_whitespace = true,
			virt_text_priority = 100,
			use_focus = false,
		},
	},
}
