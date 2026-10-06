# hyprsvn

Gio GUI version of `svn-tui`. The whole Subversion workflow of the TUI is
here unchanged — pull, status, commit (with partial hunks), revert, branches
(create / switch / merge / diff / delete), shelves, checkout revision,
conflicts, properties, commit and file history, revision tree — because the
TUI's state machine and svn logic run as they are behind a small Bubble Tea
compatible shim (`internal/tea`, `internal/textinput`, `internal/viewport`).
Only the drawing is new, and it is all widgets: lists are clickable rows
(double click = Enter), the status-bar hints are buttons, inputs are real
text fields, and the text screens are built from the structured data the
logic keeps instead of its escape-colored strings:

- **Commit history / file history**: a commit list (revision, author, date,
  message, number of paths); Enter or a double click expands the changed
  paths, `d` opens the commit diff, `/` filters by message, author, path or
  revision. The log comes from svn-tui's incremental on-disk log cache.
- **Revision tree**: a GitKraken-style graph. One lane per trunk, branch and
  tag, one row per commit (newest first), branch-out edges from the parent
  lane at the copy revision, merge-back edges into the target, name pills on
  the newest and the creating commit, hollow dots for tags. The last 250 log
  entries load first; the **Load full history** button (or `a`) loads the
  whole log. `d` / double click opens the commit's diff against the
  repository root, so branch and tag commits show their changes too.
- **Diff**: side-by-side rows with line numbers, CR markers, added / removed
  / changed row tints and a change map on the right; Alt+↑/↓ jumps between
  changes. Unified diffs, svn output and result logs are colored by line.
- **Commit screen drag & drop**: drag the selected files (or the file under
  the pointer) onto a hyprcommander window. On release hyprsvn asks
  Hyprland which window is under the cursor and sends the file list to
  hyprcommander's socket (`$XDG_RUNTIME_DIR/hyprcommander.sock`); the
  files are created in the pane under the cursor with their directories
  relative to the common parent of the dropped set (a single file lands
  under its own name). Wayland has no cross-app drag for Gio, hence the
  socket.

hyprsvn has its own config (`~/.config/hyprsvn/repo.txt`,
`~/.config/hyprsvn/ignore.txt`, log cache in `~/.cache/hyprsvn`), copied
once from svn-tui's on 2026-09-25; the two no longer share settings. Shelves
stay repository-local in `.svn-tui-shelves`. When a repository uses `credential_ref`, the window opens
with a vault-password prompt and resolves it through `hyprvault credential`
(or `gopass`).

Keys are the TUI's (see the svn-tui README); `Ctrl+P` opens the command
palette, `Ctrl+Q` quits from anywhere. The `merger` TUI is opened in a kitty
window for conflicts and read-only comparisons; Meld is used directly.
Follows the wallbash palette live; `TUI_THEME=catppuccin` forces Catppuccin
Mocha, `TUI_DIFF_THEME` works as in svn-tui.

Build: `./install-local.sh` (uses `-tags novulkan`).

## Security note

A `password=` line in `repo.txt` is handed to svn through stdin
(`--password-from-stdin`), so it never appears on the command line. The
preferred setup is `credential_ref=` with the password kept in hyprvault:
then only the vault password is ever typed, and only into hyprsvn.

## Dependencies

### Go modules

- `gioui.org` v0.10 — GUI toolkit
- `github.com/charmbracelet/lipgloss`, `github.com/muesli/termenv` — kept for the colorized text the SVN logic still produces

### Build

- Go 1.27 (see `go.mod`), a C compiler and `pkg-config` (Gio uses CGO on Linux)
- Wayland: `wayland`, `wayland-protocols`, `libxkbcommon`
- X11 fallback headers (Gio builds both backends): `libx11`, `libxcursor`, `libxfixes`
- OpenGL ES / EGL from `mesa`; Vulkan is disabled with `-tags novulkan`, so no Vulkan headers are needed

### Runtime

- A Wayland compositor (developed on Hyprland); the `.desktop` file uses `StartupWMClass`
- Fonts: JetBrainsMono Nerd Font Propo and Mono in `~/.local/share/fonts/JetBrainsMono/` (HyDE ships them; the Go fonts are the fallback), Noto Sans Symbols / Symbols2 / Noto Sans Mono as glyph fallback (optional)
- HyDE wallbash palette from `~/.cache/hyde/wallbash/shell-colors` (optional; without it, or with `TUI_THEME=catppuccin`, Catppuccin Mocha is used)
- `svn` and `svnversion` (Subversion command line); `svnadmin` only in the tests
- `patch` for partial (hunk) commits
- `hyprmerge` for diffs, merges and conflicts (falls back to the `merger` TUI in a `kitty` window; `meld` optional)
- `hyprvault` (or `gopass`) in PATH for `credential_ref` repositories
- `hyprctl` (Hyprland) and a running `hyprcommander` for dragging commit files into a hyprcommander pane

## License

MIT, see `LICENSE`.
