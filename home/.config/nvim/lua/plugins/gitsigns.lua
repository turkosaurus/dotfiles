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
	-- leading space pads the glyph off the window edge
	signs = {
		add = { text = " ✚" },
		change = { text = " ≈" },
		delete = { text = " ✖" },
		topdelete = { text = " ✖" },
		changedelete = { text = " ✹" },
	}
end

-- Inline diff: changed lines highlighted, deleted lines shown as
-- virtual lines. word_diff stays off: stacked on syntax highlighting
-- it gets too busy (:Gitsigns preview_hunk shows word detail). The
-- buffer is the real working-tree file, so edits apply live. The
-- toggles flip global gitsigns config, so close must undo them.
local function inline_diff_enable(base)
	local gs = require("gitsigns")
	if base then
		-- global: repo view opens more buffers later, and they all
		-- need to diff against the same base
		gs.change_base(base, true)
	end
	gs.toggle_linehl(true)
	gs.toggle_deleted(true)
end

local function git(args, cwd)
	local res = vim.system(vim.list_extend({ "git" }, args), { cwd = cwd }):wait()
	if res.code ~= 0 then
		return nil, vim.trim(res.stderr or "")
	end
	return vim.split(vim.trim(res.stdout or ""), "\n", { trimempty = true })
end

-- Repo-view state while the file panel is open: root, file list,
-- current index, and the file buffers given <tab> maps (cleaned up
-- on close).
local panel_state = nil
local panel_nav

-- Per-file +added/-deleted counts for the panel.
local function diff_stats(root, base)
	local args = { "diff", "--numstat" }
	if base then
		table.insert(args, base)
	end
	local stats = {}
	for _, l in ipairs(git(args, root) or {}) do
		local a, d, p = l:match("^(%S+)%s+(%S+)%s+(.*)$")
		if p then
			stats[p] = { a, d }
		end
	end
	return stats
end

-- Fold unchanged regions, keeping CONTEXT lines around each hunk. A
-- closed fold renders as one full-width rule line with the count
-- (vim folds are single-line, so this stands in for a rule/text/rule
-- sandwich). <cr>/za reopens one, zR all.
local CONTEXT = 5

function _G.InlineDiffFoldText()
	local n = vim.v.foldend - vim.v.foldstart + 1
	return "─ ⋯ " .. n .. " unchanged lines ⋯ "
end

local function apply_context_folds(buf)
	local hunks = require("gitsigns").get_hunks(buf) or {}
	if #hunks == 0 then
		-- file may have just been restored to base state: drop folds
		if vim.w.inline_diff_folds then
			vim.cmd("silent! normal! zE")
		end
		return
	end
	local last = vim.api.nvim_buf_line_count(buf)
	local ranges = {}
	for _, h in ipairs(hunks) do
		-- delete-only hunks have count 0 and can anchor at line 0
		local anchor = math.max(h.added.start, 1)
		table.insert(ranges, {
			math.max(1, anchor - CONTEXT),
			math.min(last, anchor + math.max(h.added.count, 1) - 1 + CONTEXT),
		})
	end
	table.sort(ranges, function(a, b)
		return a[1] < b[1]
	end)
	local folds, prev = {}, 0
	for _, r in ipairs(ranges) do
		if r[1] - 1 - prev >= 2 then
			table.insert(folds, { prev + 1, r[1] - 1 })
		end
		prev = math.max(prev, r[2])
	end
	if last - prev >= 2 then
		table.insert(folds, { prev + 1, last })
	end
	-- the window may carry treesitter folding (fdm=expr, nofoldenable);
	-- save it so close can restore, then switch to manual closed folds
	if not vim.w.inline_diff_folds then
		vim.w.inline_diff_folds = {
			foldmethod = vim.wo.foldmethod,
			foldenable = vim.wo.foldenable,
			foldlevel = vim.wo.foldlevel,
			foldtext = vim.wo.foldtext,
			fillchars = vim.wo.fillchars,
		}
	end
	vim.wo.foldmethod = "manual"
	vim.wo.foldenable = true
	vim.wo.foldlevel = 0
	vim.wo.foldtext = "v:lua.InlineDiffFoldText()"
	vim.wo.fillchars = "fold:─"
	vim.cmd("silent! normal! zE")
	for _, f in ipairs(folds) do
		vim.cmd(("silent! %d,%dfold"):format(f[1], f[2]))
	end
end

-- Saving in the view changes the hunk set, so refresh the context
-- folds once gitsigns settles; otherwise a new hunk can hide inside
-- a collapsed fold (and open folds re-collapse to fresh context).
vim.api.nvim_create_autocmd("BufWritePost", {
	callback = function(ev)
		if not vim.w.inline_diff_folds then
			return
		end
		vim.defer_fn(function()
			if vim.api.nvim_get_current_buf() == ev.buf and vim.w.inline_diff_folds then
				apply_context_folds(ev.buf)
			end
		end, 200)
	end,
})

-- Land the cursor on the first change once gitsigns has attached
-- and computed hunks (async, hence the deferred retry).
local function first_hunk(buf, tries)
	vim.defer_fn(function()
		if vim.api.nvim_get_current_buf() ~= buf then
			return
		end
		if require("gitsigns").get_hunks(buf) then
			apply_context_folds(buf)
			require("gitsigns").nav_hunk("first", { navigation_message = false })
		elseif (tries or 0) < 10 then
			first_hunk(buf, (tries or 0) + 1)
		end
	end, 100)
end

-- :edit that survives E325 (swap file exists): vim's interactive
-- swap prompt can't run inside a lua mapping, so ask via confirm()
-- and retry with the choice preselected. Returns true when the file
-- is now the current buffer.
local function swap_safe_edit(path)
	local ok, err = pcall(vim.cmd.edit, vim.fn.fnameescape(path))
	if ok then
		return true
	end
	if not tostring(err):find("E325") then
		error(err)
	end
	local choice = vim.fn.confirm(
		"Swap file exists for " .. vim.fn.fnamemodify(path, ":t") .. " (crashed or concurrent session).",
		"&Edit anyway\n&Delete swap\n&Read-only\n&Skip",
		2
	)
	local sc = ({ "e", "d", "o" })[choice]
	if not sc then
		return false
	end
	vim.api.nvim_create_autocmd("SwapExists", {
		once = true,
		callback = function()
			vim.v.swapchoice = sc
		end,
	})
	return pcall(vim.cmd.edit, vim.fn.fnameescape(path))
end

-- Open panel_state.files[idx] in the file window (right of the
-- panel), give it <tab>/<s-tab> next/prev-file maps, and sync the
-- panel cursor.
local function panel_open_file()
	local st = panel_state
	if vim.b.inline_diff_panel then
		vim.cmd("wincmd l")
	end
	local path = st.root .. "/" .. st.files[st.idx]
	local cur = vim.api.nvim_buf_get_name(0)
	-- skip :edit when the target is already showing (e.g. entering
	-- the view from that file, possibly via a symlinked path)
	local same = cur ~= "" and vim.uv.fs_realpath(cur) == vim.uv.fs_realpath(path)
	if not same and not swap_safe_edit(path) then
		return
	end
	local buf = vim.api.nvim_get_current_buf()
	first_hunk(buf)
	if not st.mapped[buf] then
		st.mapped[buf] = true
		vim.keymap.set("n", "<tab>", function()
			panel_nav(1)
		end, { buffer = buf, desc = "Next changed file" })
		vim.keymap.set("n", "<s-tab>", function()
			panel_nav(-1)
		end, { buffer = buf, desc = "Prev changed file" })
		vim.keymap.set("n", "<cr>", function()
			if vim.fn.foldclosed(".") ~= -1 then
				vim.cmd("normal! zv")
			end
		end, { buffer = buf, desc = "Open fold" })
	end
	for _, w in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
		if vim.b[vim.api.nvim_win_get_buf(w)].inline_diff_panel then
			vim.api.nvim_win_set_cursor(w, { st.idx, 0 })
		end
	end
end

panel_nav = function(step)
	local st = panel_state
	if not st then
		return
	end
	st.idx = (st.idx - 1 + step) % #st.files + 1
	panel_open_file()
end

-- Panel display form of a path, at most `max` cells. Repo-root files
-- show bare; anything in a directory gets a ".../" prefix, dropping
-- whole leading directories until it fits (".../lua/plugins/foo.lua").
-- A lone overlong name gets its front hard-chopped ("...ng-name.ext").
local function shorten(path, max)
	local tail = path
	while tail:find("/") do
		if #tail + 4 <= max then
			return ".../" .. tail
		end
		tail = tail:match("^[^/]+/(.*)$")
	end
	if tail == path and #tail <= max then
		return tail
	end
	if #tail + 4 <= max then
		return ".../" .. tail
	end
	return "..." .. tail:sub(#tail - (max - 3) + 1)
end

-- Narrow left panel listing changed files with +/- counts; <cr>
-- opens the file under the cursor, <tab>/<s-tab> cycle files. The
-- right window renders each file's inline diff since the gitsigns
-- toggles are global. Width = 2 pad + 30 path + stats zone.
local function file_panel(base)
	local st = panel_state
	local stats = diff_stats(st.root, base)
	vim.cmd("topleft 42vnew")
	local buf = vim.api.nvim_get_current_buf()
	-- pad paths off the screen edge; <cr>/<tab> go by line number,
	-- not line content, so padding and truncation are cosmetic only
	local lines = {}
	for i, f in ipairs(st.files) do
		lines[i] = "  " .. shorten(f, 30)
	end
	vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
	local ns = vim.api.nvim_create_namespace("inline_diff_panel")
	for i, f in ipairs(st.files) do
		local s = stats[f]
		if s then
			vim.api.nvim_buf_set_extmark(buf, ns, i - 1, 0, {
				virt_text = { { "+" .. s[1] .. " ", "GitSignsAdd" }, { "-" .. s[2], "GitSignsDelete" } },
				virt_text_pos = "right_align",
			})
		end
	end
	vim.bo[buf].buftype = "nofile"
	vim.bo[buf].bufhidden = "wipe"
	vim.bo[buf].swapfile = false
	vim.bo[buf].modifiable = false
	vim.b[buf].inline_diff_panel = true
	vim.wo.winfixwidth = true
	vim.wo.number = false
	vim.wo.relativenumber = false
	vim.wo.signcolumn = "no"
	vim.wo.cursorline = true
	vim.keymap.set("n", "<cr>", function()
		panel_state.idx = vim.api.nvim_win_get_cursor(0)[1]
		panel_open_file()
	end, { buffer = buf, desc = "Open file diff" })
	vim.keymap.set("n", "<tab>", function()
		panel_nav(1)
	end, { buffer = buf, desc = "Next changed file" })
	vim.keymap.set("n", "<s-tab>", function()
		panel_nav(-1)
	end, { buffer = buf, desc = "Prev changed file" })
end

-- Cleanup shared by <leader>gq and the TabClosed autocmd, so ZZ /
-- :q / :x on the view's windows also restore normal rendering.
local view_tab
local function inline_diff_reset()
	local gs = require("gitsigns")
	gs.toggle_linehl(false)
	gs.toggle_deleted(false)
	gs.change_base(nil, true)
	if panel_state then
		for b in pairs(panel_state.mapped) do
			if vim.api.nvim_buf_is_valid(b) then
				pcall(vim.keymap.del, "n", "<tab>", { buffer = b })
				pcall(vim.keymap.del, "n", "<s-tab>", { buffer = b })
				pcall(vim.keymap.del, "n", "<cr>", { buffer = b })
			end
		end
		panel_state = nil
	end
	-- clear context folds in windows the tab close didn't take out,
	-- restoring each window's saved fold setup
	for _, w in ipairs(vim.api.nvim_list_wins()) do
		local saved = vim.w[w].inline_diff_folds
		if saved then
			vim.api.nvim_win_call(w, function()
				vim.cmd("silent! normal! zE")
				vim.wo.foldmethod = saved.foldmethod
				vim.wo.foldenable = saved.foldenable
				vim.wo.foldlevel = saved.foldlevel
				vim.wo.foldtext = saved.foldtext
				vim.wo.fillchars = saved.fillchars
				vim.w.inline_diff_folds = nil
			end)
		end
	end
	view_tab = nil
end

vim.api.nvim_create_autocmd("TabClosed", {
	callback = function()
		if view_tab and not vim.api.nvim_tabpage_is_valid(view_tab) then
			inline_diff_reset()
		end
	end,
})

-- The view is always panel + file: left panel lists every file
-- changed against the base, right window shows one inline. Entering
-- from a changed file buffer selects that file; from anywhere else
-- (netrw, neo-tree, an empty buffer) the first changed file opens.
-- `launch` (the :Diff CLI entry) reuses the current tab instead of
-- opening a new one.
local function inline_diff(base, launch)
	local cur = vim.api.nvim_buf_get_name(0)
	local is_file = cur ~= "" and vim.fn.filereadable(cur) == 1
	local cwd = is_file and vim.fs.dirname(cur) or vim.fn.getcwd()
	local args = { "diff", "--name-only" }
	if base then
		table.insert(args, base)
	end
	local files, err = git(args, cwd)
	if not files then
		vim.notify("inline diff: " .. err, vim.log.levels.WARN)
		return
	end
	if #files == 0 then
		vim.notify("no changes", vim.log.levels.INFO)
		return
	end
	local root = (git({ "rev-parse", "--show-toplevel" }, cwd) or { cwd })[1]
	local idx
	if is_file then
		-- realpath both sides: buffers are often opened through
		-- symlinks (e.g. ~/.config -> dotfiles)
		local real = vim.uv.fs_realpath(cur)
		for i, f in ipairs(files) do
			if vim.uv.fs_realpath(root .. "/" .. f) == real then
				idx = i
				break
			end
		end
	end
	if not launch then
		-- tab split keeps the current buffer loaded in the new tab
		-- when it is one of the changed files
		vim.cmd(idx and "tab split" or "tabnew")
		vim.t.inline_diff = true
		view_tab = vim.api.nvim_get_current_tabpage()
	end
	panel_state = { root = root, files = files, idx = idx or 1, mapped = {} }
	inline_diff_enable(base)
	file_panel(base)
	panel_open_file()
end

-- Launch straight into the view: nvim +Diff [file], or +DiffMain
-- to diff against main without having to quote "+Diff main". Both
-- run after the first buffer loads, so gitsigns (event BufEnter)
-- is already attached.
vim.api.nvim_create_user_command("Diff", function(opts)
	inline_diff(opts.args ~= "" and opts.args or nil, true)
end, { nargs = "?", desc = "Inline diff" })
vim.api.nvim_create_user_command("DiffMain", function()
	inline_diff("main", true)
end, { desc = "Inline diff vs main" })

local function diff_close()
	if not (vim.t.inline_diff or require("gitsigns.config").config.linehl) then
		vim.cmd("DiffviewClose")
		return
	end
	local in_view_tab = vim.t.inline_diff
	inline_diff_reset()
	if in_view_tab then
		vim.cmd("tabclose")
	else
		-- launch mode made no tab; just drop the panel if present
		for _, w in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
			if vim.b[vim.api.nvim_win_get_buf(w)].inline_diff_panel then
				vim.api.nvim_win_close(w, true)
			end
		end
	end
end

return {
	"lewis6991/gitsigns.nvim",
	event = "BufEnter",
	keys = {
		{ "<leader>gb", "<cmd>Gitsigns toggle_current_line_blame<cr>", desc = "Toggle git blame line" },
		{
			"<leader>gd",
			function()
				inline_diff()
			end,
			desc = "Inline diff vs index",
		},
		{
			"<leader>gD",
			function()
				inline_diff("main")
			end,
			desc = "Inline diff vs main",
		},
		{ "<leader>gq", diff_close, desc = "Close diff" },
		{ "<leader>gQ", "<cmd>confirm qa<cr>", desc = "Quit vim" },
		-- shadow native gQ (Ex mode): a missed leader chord lands
		-- there and it looks like broken cruft
		{ "gQ", "<cmd>confirm qa<cr>", desc = "Quit vim" },
		{
			"]c",
			function()
				if vim.wo.diff then
					vim.cmd.normal({ "]c", bang = true })
				else
					require("gitsigns").nav_hunk("next", { wrap = true })
				end
			end,
			desc = "Next hunk",
		},
		{
			"[c",
			function()
				if vim.wo.diff then
					vim.cmd.normal({ "[c", bang = true })
				else
					require("gitsigns").nav_hunk("prev", { wrap = true })
				end
			end,
			desc = "Prev hunk",
		},
		{
			"<leader>gl",
			function()
				require("gitsigns").setqflist("all")
			end,
			desc = "All hunks to quickfix",
		},
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
