return {
	"MeanderingProgrammer/render-markdown.nvim",
	dependencies = { "nvim-treesitter/nvim-treesitter", "nvim-mini/mini.nvim" },
	ft = { "markdown" },
	keys = {
		{ "<leader>md", "<cmd>RenderMarkdown toggle<cr>", desc = "Toggle Markdown Rendering" },
	},
	opts = {
		-- Heading styling
		heading = {
			enabled = true,
			backgrounds = {
				"RenderMarkdownH1Bg",
				"RenderMarkdownH2Bg",
				"RenderMarkdownH3Bg",
				"RenderMarkdownH4Bg",
				"RenderMarkdownH5Bg",
				"RenderMarkdownH6Bg",
			},
			foregrounds = {
				"RenderMarkdownH1",
				"RenderMarkdownH2",
				"RenderMarkdownH3",
				"RenderMarkdownH4",
				"RenderMarkdownH5",
				"RenderMarkdownH6",
			},
			icons = { "# ", "## ", "### ", "#### ", "##### ", "###### " },
			sign = { enabled = false },
			position = "overlay",
			width = "full",
			border = false,
		},

		-- Code block styling
		code = {
			enabled = true,
			width = "full", -- 'block' or 'full'
			border = "thick", -- 'thick', 'thin', or 'none'
			language_name = false, -- Hide language name
			disable_background = {}, -- List languages to not highlight, e.g. { 'lua', 'python' }
		},

		-- Bullet/list styling
		bullet = {
			enabled = true,
			icons = { "•", "◦", "▪", "▫" }, -- Simpler bullets
		},

		-- Checkbox styling
		checkbox = {
			enabled = true,
			unchecked = { icon = "-  󰄱 " },
			checked   = { icon = "-  󰱒 " },
		},

		-- Disable specific features
		dash = { enabled = true }, -- Thematic breaks
		quote = { enabled = true }, -- Block quotes
		pipe_table = { enabled = true }, -- Tables
		link = {
			enabled = true,
			image = "󰥶 ",
			email = "󰀓 ",
			hyperlink = "󰌹 ",
			highlight = "RenderMarkdownLink",
			custom = {
				-- Built-in: GitHub, Discord, Wikipedia, YouTube, etc.
				-- Add your own:
				{
					pattern = "example%.com",
					icon = "󰖟 ",
					highlight = "Special",
				},
			},
		},

		-- Footnote links [^1]
		footnote = {
			enabled = true,
			icon = "󰯔 ",
			superscript = true, -- Render as superscript
		},

		-- WikiLinks [[Page Name]]
		wiki_link = {
			enabled = true,
			icon = "󱗖 ",
			highlight = "RenderMarkdownWikiLink",
		},
	},
}
