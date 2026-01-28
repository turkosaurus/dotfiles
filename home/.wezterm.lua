local wezterm = require("wezterm")
local config = {}

config.color_scheme = "Tokyo Night"
config.font = wezterm.font_with_fallback({
	-- macOS
	{
		family = "RobotoMono Nerd Font Mono",
		weight = "Regular",
		stretch = "Normal",
		style = "Normal",
	},
	-- linux
	{
		family = "Roboto Mono Nerd Font Mono",
		weight = "Regular",
		stretch = "Normal",
		style = "Normal",
	},
})
config.font_size = 16

-- -- ~/.wezterm.lua
-- wezterm.on("format-window-title", function(tab, pane, tabs, panes, config)
-- 	local title = tab.active_pane.title
-- 	local window_width = wezterm.gui.screens().main.width
-- 	local title_length = #title
-- 	local padding_needed = math.max(0, math.floor((window_width / config.font_size / 2) - (title_length / 2)))
-- 	local left_padding = string.rep("  ", padding_needed)
--
-- 	return left_padding .. title
-- end)

config.keys = {
	{
		key = "F11",
		action = wezterm.action.ToggleFullScreen,
	},
	{
		key = "Enter",
		mods = "SHIFT",
		action = wezterm.action.SendString("\x1b\r"),
	},
	{
		key = "f",
		mods = "CTRL|SHIFT",
		action = wezterm.action.ToggleFullScreen,
	},
}

-- config.tab_bar_at_bottom = true
config.use_fancy_tab_bar = false
config.hide_tab_bar_if_only_one_tab = true
-- config.window_frame = {
-- 	border_left_width = "0.5cell",
-- 	border_right_width = "0.5cell",
-- 	border_bottom_height = "0.5cell",
-- 	border_top_height = "0.25cell",
-- 	border_left_color = "grey",
-- 	border_right_color = "grey",
-- 	border_bottom_color = "grey",
-- 	border_top_color = "grey",
-- }
config.window_padding = {
	left = 0,
	right = 0,
	top = 30, -- notch
	bottom = 0,
}
config.window_decorations = "TITLE | RESIZE"

config.default_cursor_style = "BlinkingBlock"
config.cursor_blink_rate = 340
config.force_reverse_video_cursor = true
config.max_fps = 60
config.animation_fps = 60
config.cursor_blink_ease_in = "Constant"
config.cursor_blink_ease_out = "Constant"
config.term = "wezterm"

return config
