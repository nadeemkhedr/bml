#!/usr/bin/env bash
#
# Open bml as a small, centered popup window in Ghostty — using Ghostty's
# AppleScript API only (no `ghostty` CLI / PATH dependency).
#
# Approach: open a normal Ghostty window, center it, then *type* `bml; exit`
# into its shell. bml runs; when it exits, `exit` closes the window the normal
# way — no "Process exited, press any key" hold (that only affects surfaces
# launched via Ghostty's `command`, which can't run anyway).
#
# Bind this to a global shortcut (Raycast, Karabiner, skhd, macOS Shortcuts, …).
# Whatever runs it needs Accessibility permission (to size/move the window) plus
# the Automation permission bml already uses for the browser.

BML="${BML:-/usr/local/bin/bml}"
WIDTH="${BML_POPUP_WIDTH:-760}"   # window width in pixels
HEIGHT="${BML_POPUP_HEIGHT:-460}" # window height in pixels

osascript - "$BML" "$WIDTH" "$HEIGHT" <<'APPLESCRIPT'
use framework "AppKit"
use scripting additions

on run argv
	set bmlPath to item 1 of argv
	set winW to (item 2 of argv) as integer
	set winH to (item 3 of argv) as integer

	-- How many Ghostty windows exist now, so we can find the new one.
	set oldCount to 0
	tell application "System Events"
		if exists process "Ghostty" then set oldCount to (count of windows of process "Ghostty")
	end tell

	-- Open a fresh Ghostty window (normal shell) and grab its terminal.
	tell application "Ghostty"
		activate
		set myWin to (new window)
		set myPane to focused terminal of selected tab of myWin
	end tell

	-- Primary screen height (for the Cocoa→Accessibility y-flip) and the active
	-- screen's visible frame (where we center).
	set primaryH to 0
	repeat with s in (current application's NSScreen's screens() as list)
		set fr to s's frame()
		if (item 1 of item 1 of fr) = 0 and (item 2 of item 1 of fr) = 0 then
			set primaryH to (item 2 of item 2 of fr)
		end if
	end repeat
	set vf to (current application's NSScreen's mainScreen()'s visibleFrame())
	set vx to item 1 of item 1 of vf
	set vy to item 2 of item 1 of vf
	set vw to item 1 of item 2 of vf
	set vh to item 2 of item 2 of vf

	-- Size + center the new window (it is frontmost) via System Events.
	tell application "System Events"
		tell process "Ghostty"
			set theWin to missing value
			repeat 60 times
				if (count of windows) > oldCount then
					set theWin to window 1
					exit repeat
				end if
				delay 0.05
			end repeat
			if theWin is not missing value then
				set size of theWin to {winW, winH}
				set {w, h} to size of theWin
				set cx to (vx + (vw - w) / 2) as integer
				set seY to (primaryH - (vy + (vh - h) / 2) - h) as integer
				set position of theWin to {cx, seY}
			end if
		end tell
	end tell

	-- Run bml, then close the window by exiting the shell. `clear` hides the
	-- prompt + typed command; the window is already centered above, so this
	-- fires as soon as the pane can accept input (kept minimal to reduce the
	-- moment of visible shell before bml takes over the alternate screen).
	delay 0.05
	tell application "Ghostty"
		input text ("clear; " & bmlPath & "; exit") to myPane
		send key "enter" to myPane
	end tell
end run
APPLESCRIPT
