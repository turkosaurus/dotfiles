-- Neo-tree is a Neovim plugin to browse the file system
-- https://github.com/nvim-neo-tree/neo-tree.nvim

return {
    "nvim-neo-tree/neo-tree.nvim",
    version = "*",
    dependencies = {
        "nvim-lua/plenary.nvim",
        "nvim-tree/nvim-web-devicons", -- not strictly required, but recommended
        "MunifTanjim/nui.nvim",
    },
    cmd = "Neotree",
    keys = {
        { "\\", ":Neotree reveal<CR>", desc = "NeoTree reveal", silent = true },
        {
            "|",
            ":Neotree document_symbols<CR>",
            desc = "NeoTree Symbols",
            silent = true,
        }, -- Add shortcut for symbols
    },
    opts = {
        sources = {
            "filesystem",
            "document_symbols", -- Add the document_symbols source
        },
        source_selector = {
            winbar = true, -- Show source selector in the window bar
            statusline = false, -- Disable source selector in the statusline
            sources = {
                { source = "filesystem", display_name = " Files" },
                { source = "document_symbols", display_name = "󰊕 Symbols" },
            },
        },
        filesystem = {
            follow_cursor = true, -- Automatically refresh and follow the cursor
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
            follow_cursor = true, -- Automatically refresh and follow the cursor
            kinds = { -- Customize symbol kinds and icons
                Class = { icon = "󰠱", hl = "TSClass" },
                Function = { icon = "󰊕", hl = "TSFunction" },
                Variable = { icon = "󰀫", hl = "TSVariable" },
                -- Add more kinds as needed
            },
            window = {
                position = "right", -- Ensure the document_symbols source is on the right
                width = 24,
                auto_expand_width = true,
                mappings = {
                    ["|"] = "close_window",
                },
            },
        },
        default_component_configs = {
            icon = {
                folder_closed = "[+]", -- Closed folder icon
                folder_open = "[-]", -- Open folder icon
                folder_empty = "[ ]", -- Empty folder icon
                default = "-", -- Default file icon
            },
        },
    },
}
