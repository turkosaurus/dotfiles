-- Soft-wrap plan files only. plan.toml holds hand-wrapped multi-line
-- prose (tasks[], pr.comment bodies); wrap makes over-long lines
-- readable without touching the file. Ordinary TOML keeps nowrap.
if vim.fn.expand("%:t") == "plan.toml" then
	vim.opt_local.wrap = true
	vim.opt_local.linebreak = true
end
