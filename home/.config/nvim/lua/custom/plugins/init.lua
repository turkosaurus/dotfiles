-- You can add your own plugins here or in other files in this directory!
--  I promise not to create any merge conflicts in this directory :)
--
-- See the kickstart.nvim README for more information

local lspconfig = require "lspconfig"
local configs = require "lspconfig/configs"

if not configs.golangcilsp then
    configs.golangcilsp = {
        default_config = {
            cmd = { "golangci-lint-langserver" },
            root_dir = lspconfig.util.root_pattern(".git", "go.mod"),
            init_options = {
                command = {
                    "golangci-lint",
                    "run",
                    "--output.json.path",
                    "stdout",
                    "--show-stats=false",
                    "--issues-exit-code=1",
                },
            },
        },
    }
end
lspconfig.golangci_lint_ls.setup {
    filetypes = { "go", "gomod" },
}
