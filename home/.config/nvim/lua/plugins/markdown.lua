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

			-- Background colors (one per level)
			backgrounds = {
				"RenderMarkdownH1Bg", -- # Level 1
				"RenderMarkdownH2Bg", -- ## Level 2
				"RenderMarkdownH3Bg", -- ### Level 3
				"RenderMarkdownH4Bg", -- #### Level 4
				"RenderMarkdownH5Bg", -- ##### Level 5
				"RenderMarkdownH6Bg", -- ###### Level 6
			},

			-- Foreground colors (one per level)
			foregrounds = {
				"RenderMarkdownH1", -- # Level 1
				"RenderMarkdownH2", -- ## Level 2
				"RenderMarkdownH3", -- ### Level 3
				"RenderMarkdownH4", -- #### Level 4
				"RenderMarkdownH5", -- ##### Level 5
				"RenderMarkdownH6", -- ###### Level 6
			},
			icons = { "󰲡 ", "󰲣 ", "󰲥 ", "󰲧 ", "󰲩 ", "󰲫 " }, -- Or use '#', '##', etc.
			position = "overlay", -- 'overlay', 'inline', or 'right'
			width = "full", -- 'full' or 'block'
			border = true, -- Remove border lines above/below headings
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
			unchecked = { icon = "󰄱 " },
			checked = { icon = "󰱒 " },
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
