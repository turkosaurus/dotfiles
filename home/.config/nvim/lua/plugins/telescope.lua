return {
	"nvim-telescope/telescope.nvim",
	event = "VimEnter",
	dependencies = {
		"nvim-lua/plenary.nvim",
		{
			"nvim-telescope/telescope-fzf-native.nvim",
			build = "make",
			cond = function()
				return vim.fn.executable("make") == 1
			end,
		},
		{ "nvim-telescope/telescope-ui-select.nvim" },
		{ "nvim-tree/nvim-web-devicons", enabled = vim.g.have_nerd_font },
	},
	config = function()
		require("telescope").setup({
			defaults = {
				vimgrep_arguments = {
					"rg",
					"--color=never",
					"--no-heading",
					"--with-filename",
					"--line-number",
					"--column",
					"--smart-case",
					"--hidden",
				},
				file_ignore_patterns = { ".git/" },
				hidden = true,
			},
			pickers = {
				find_files = { hidden = true },
				live_grep = {
					additional_args = function()
						return { "--hidden" }
					end,
				},
			},
			extensions = {
				["ui-select"] = {
					require("telescope.themes").get_dropdown(),
				},
			},
		})

		pcall(require("telescope").load_extension, "fzf")
		pcall(require("telescope").load_extension, "ui-select")

		local builtin = require("telescope.builtin")
		vim.keymap.set("n", "<leader>sh", builtin.help_tags, { desc = "search help" })
		vim.keymap.set("n", "<leader>sk", builtin.keymaps, { desc = "search keymaps" })
		vim.keymap.set("n", "<leader>sf", builtin.find_files, { desc = "search files" })
		vim.keymap.set("n", "<leader>ss", builtin.builtin, { desc = "search select telescope" })
		vim.keymap.set("n", "<leader>sw", builtin.grep_string, { desc = "search current word" })
		vim.keymap.set("n", "<leader>sg", builtin.live_grep, { desc = "search by grep" })
		vim.keymap.set("n", "<leader>sd", builtin.diagnostics, { desc = "search diagnostics" })
		vim.keymap.set("n", "<leader>sr", builtin.resume, { desc = "search resume" })
		vim.keymap.set("n", "<leader>s.", builtin.oldfiles, { desc = "search recent files" })
		vim.keymap.set("n", "<leader><leader>", builtin.buffers, { desc = "find existing buffers" })

		vim.keymap.set("n", "<leader>/", function()
			builtin.current_buffer_fuzzy_find(require("telescope.themes").get_dropdown({
				winblend = 10,
				previewer = false,
			}))
		end, { desc = "search in current buffer" })

		vim.keymap.set("n", "<leader>s/", function()
			builtin.live_grep({
				grep_open_files = true,
				prompt_title = "Live Grep in Open Files",
			})
		end, { desc = "search in open files" })

		vim.keymap.set("n", "<leader>sn", function()
			builtin.find_files({ cwd = vim.fn.stdpath("config") })
		end, { desc = "search neovim files" })
	end,
}
