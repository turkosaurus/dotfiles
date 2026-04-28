vim.opt_local.spell = true
vim.opt_local.spelllang = "en"
vim.opt_local.wrap = true
vim.opt_local.linebreak = true
vim.opt_local.formatoptions:remove("t")

-- Kill switch: disable LLM/predictive text
vim.b.copilot_suggestion_auto_trigger = false
vim.b.completion = false
