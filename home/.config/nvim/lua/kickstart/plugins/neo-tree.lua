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
    },
    opts = {
        filesystem = {
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
