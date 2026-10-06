// Gio front end of hyprsvn. The Bubble Tea state machine (Model, app.go,
// cmds.go) is kept as is; this file runs it: keys become Bubble Tea key
// strings, commands run in goroutines and post their messages back, and the
// screens are drawn from the Model's fields with real widgets.
package ui

import (
	"image"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/hussaratkuro/hyprsvn/internal/credential"
	"github.com/hussaratkuro/hyprsvn/internal/model"
	"github.com/hussaratkuro/hyprsvn/internal/svn"
	"github.com/hussaratkuro/hyprsvn/internal/tea"
	"github.com/hussaratkuro/hyprsvn/internal/theme"
)

const (
	monoFont = "JetBrainsMono Nerd Font Mono"
	monoSize = unit.Sp(13)
)

type App struct {
	w       *app.Window
	th      *material.Theme
	pal     theme.Palette
	posted  chan func()
	pending atomic.Int32

	m     Model
	ready bool // false until the vault password (if any) is resolved

	configs   []model.RepoConfig
	vaultEd   widget.Editor
	vaultBtn  widget.Clickable
	vaultErr  string
	vaultBusy bool

	// generic widgets reused by every screen
	list       widget.List
	rowClicks  []widget.Clickable
	palList    widget.List // command palette's own scroll state, swapped in by layoutPalette
	palClicks  []widget.Clickable
	palCursor  int
	hintBtns   map[string]*widget.Clickable
	infoBtn    widget.Clickable
	backBtn    widget.Clickable
	paletteBtn widget.Clickable
	locSel     widget.Selectable // header branch/location, selectable + Ctrl+C

	lastCursorScreen model.Screen
	lastCursor       int
	cols, rows       int

	hist historyView
	diff diffView
	drag dragState
}

func Run(configs []model.RepoConfig) error {
	lipgloss.SetColorProfile(termenv.TrueColor)
	tea.TerminalWrapper = wrapInTerminal
	w := new(app.Window)
	w.Option(app.Title("hyprsvn"), app.Size(unit.Dp(1180), unit.Dp(760)), app.MinSize(unit.Dp(720), unit.Dp(460)))
	a := &App{w: w, th: material.NewTheme(), posted: make(chan func(), 64), configs: configs, hintBtns: map[string]*widget.Clickable{}}
	a.list.Axis = layout.Vertical
	a.palList.Axis = layout.Vertical
	a.hist.list.Axis, a.diff.list.Axis, a.diff.graph.list.Axis = layout.Vertical, layout.Vertical, layout.Vertical
	a.drag.hoverRow, a.drag.pressRow = -1, -1
	a.vaultEd = widget.Editor{SingleLine: true, Submit: true, Mask: '•'}
	a.th.Shaper = text.NewShaper(text.WithCollection(append(loadFonts(), loadMonoFonts()...)))
	a.th.Face = nerdFont
	a.applyTheme(theme.Current())
	theme.Watch(func(p theme.Palette) { a.post(func() { a.applyTheme(p) }) })
	if !credential.Required(configs) {
		a.start(svn.BuildRepos(configs))
	}

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			a.drain()
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// wrapInTerminal opens a terminal tool (the merger TUI) in a kitty window.
func wrapInTerminal(c *exec.Cmd) *exec.Cmd {
	args := append([]string{"--class", "merger", "--title", "merger", "-e", c.Path}, c.Args[1:]...)
	k := exec.Command("kitty", args...)
	k.Dir = c.Dir
	return k
}

func loadMonoFonts() []font.FontFace {
	var faces []font.FontFace
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".local", "share", "fonts", "JetBrainsMono")
	for _, v := range []struct {
		file   string
		weight font.Weight
	}{{"Regular", font.Normal}, {"Bold", font.Bold}} {
		data, err := os.ReadFile(filepath.Join(dir, "JetBrainsMonoNerdFontMono-"+v.file+".ttf"))
		if err != nil {
			continue
		}
		face, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: monoFont, Weight: v.weight}, Face: face})
	}
	return faces
}

func (a *App) start(repos []model.Repo) {
	a.m = NewModel(repos)
	a.ready = true
	a.run(a.m.Init())
}

// post runs f on the UI goroutine before the next frame. Gio coalesces
// Invalidate calls until the app has gone idle again, so an Invalidate that
// lands while a frame is being processed is dropped; the retries below make
// sure a posted result is never left waiting for the next input event.
func (a *App) post(f func()) {
	a.posted <- f
	a.pending.Add(1)
	a.w.Invalidate()
	go func() {
		for i := 0; i < 12 && a.pending.Load() > 0; i++ {
			time.Sleep(25 * time.Millisecond)
			a.w.Invalidate()
		}
	}()
}

func (a *App) drain() {
	for {
		select {
		case f := <-a.posted:
			f()
			a.pending.Add(-1)
		default:
			return
		}
	}
}

func (a *App) applyTheme(p theme.Palette) {
	a.pal = p
	a.th.Palette = material.Palette{Bg: p.Base, Fg: p.Text, ContrastBg: p.Mauve, ContrastFg: p.OnAccent}
	applyTheme(p) // lipgloss styles still used by the text-producing logic
}

// ---- running the state machine ----

func (a *App) dispatch(msg tea.Msg) {
	switch v := msg.(type) {
	case tea.QuitMsg:
		a.w.Perform(system.ActionClose)
		return
	case tea.BatchMsg:
		for _, c := range v {
			a.run(c)
		}
		return
	}
	next, cmd := a.m.Update(msg)
	a.m = next.(Model)
	a.run(cmd)
}

func (a *App) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		if msg == nil {
			return
		}
		a.post(func() { a.dispatch(msg) })
	}()
}

func (a *App) sendKey(name string) {
	if a.ownsKey(name) {
		return
	}
	a.dispatch(tea.Key(name))
}

// ownsKey handles the keys of the screens the GUI draws itself (commit list
// with its graph, diff viewer); everything else goes to the state machine.
func (a *App) ownsKey(name string) bool {
	if a.m.commands.open {
		return false
	}
	switch a.m.screen {
	case model.ScreenHistory:
		return a.historyKey(name)
	case model.ScreenDiff:
		return a.diffKey(name)
	}
	return false
}

func teaKeyName(k key.Event) string {
	ctrl, alt, shift := k.Modifiers.Contain(key.ModCtrl), k.Modifiers.Contain(key.ModAlt), k.Modifiers.Contain(key.ModShift)
	base := ""
	switch k.Name {
	case key.NameEscape:
		return "esc"
	case key.NameReturn, key.NameEnter:
		return "enter"
	case key.NameUpArrow:
		base = "up"
	case key.NameDownArrow:
		base = "down"
	case key.NameLeftArrow:
		base = "left"
	case key.NameRightArrow:
		base = "right"
	case key.NamePageUp:
		return "pgup"
	case key.NamePageDown:
		return "pgdown"
	case key.NameHome:
		return "home"
	case key.NameEnd:
		return "end"
	case key.NameDeleteBackward:
		return "backspace"
	case key.NameDeleteForward:
		return "delete"
	case key.NameSpace:
		return " "
	case key.NameTab:
		return "tab"
	default:
		s := string(k.Name)
		if len([]rune(s)) != 1 {
			return ""
		}
		if ctrl {
			return "ctrl+" + strings.ToLower(s)
		}
		if alt {
			return "alt+" + strings.ToLower(s)
		}
		if !shift {
			return strings.ToLower(s)
		}
		return s
	}
	if alt {
		return "alt+" + base
	}
	return base
}

func (a *App) globalKeys(gtx C) {
	filters := []event.Filter{
		key.Filter{Optional: key.ModCtrl | key.ModShift | key.ModAlt | key.ModSuper},
		key.Filter{Name: key.NameTab, Optional: key.ModShift},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		k, ok := ev.(key.Event)
		if !ok || k.State != key.Press {
			continue
		}
		if !a.ready {
			continue
		}
		if name := teaKeyName(k); name != "" {
			if name == "ctrl+q" {
				a.w.Perform(system.ActionClose)
				continue
			}
			if name == "ctrl+c" {
				if sel := a.locSel.SelectedText(); sel != "" {
					gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(sel))})
					continue
				}
			}
			a.sendKey(name)
		}
	}
}

// ---- layout ----

func (a *App) charW(gtx C) int { return max(1, gtx.Sp(monoSize)*6/10) }
func (a *App) lineH(gtx C) int { return gtx.Sp(monoSize) * 15 / 10 }

func (a *App) layout(gtx C) D {
	paint.FillShape(gtx.Ops, a.pal.Base, clip.Rect{Max: gtx.Constraints.Max}.Op())
	keyArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, a)
	keyArea.Pop()

	if !a.ready {
		a.layoutVaultPassword(gtx)
		a.globalKeys(gtx)
		return D{Size: gtx.Constraints.Max}
	}

	// The state machine still sizes its diff text by columns; keep it fed.
	cols := (gtx.Constraints.Max.X - gtx.Dp(72)) / a.charW(gtx)
	rows := (gtx.Constraints.Max.Y - gtx.Dp(120)) / a.lineH(gtx)
	if cols != a.cols || rows != a.rows {
		a.cols, a.rows = cols, rows
		a.dispatch(tea.WindowSizeMsg{Width: max(40, cols), Height: max(12, rows)})
	}

	if a.inputVisible() {
		ed := a.m.input.Editor()
		a.wordJumps(gtx, ed)
		for {
			ev, ok := ed.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.SubmitEvent); ok {
				a.sendKey("enter")
			}
		}
		if !gtx.Focused(ed) {
			gtx.Execute(key.FocusCmd{Tag: ed})
		}
	} else if gtx.Focused(a.m.input.Editor()) {
		gtx.Execute(key.FocusCmd{Tag: nil})
	}
	if a.infoBtn.Clicked(gtx) && a.m.screen != model.ScreenRepoSelect {
		a.m.showInfo = !a.m.showInfo
	}
	if a.paletteBtn.Clicked(gtx) && !a.m.commands.open {
		a.m.openCommandPalette()
	}
	if a.backBtn.Clicked(gtx) {
		a.sendKey("esc")
	}

	a.layoutScreen(gtx)
	if a.m.commands.open {
		a.layoutPalette(gtx)
	}
	a.globalKeys(gtx)
	return D{Size: gtx.Constraints.Max}
}

// wordJumps handles Ctrl+←/→ (with Shift to extend the selection) before the
// editor sees them: Gio's Editor treats Ctrl as its shortcut modifier on Linux
// and drops the word movement it would otherwise do.
func (a *App) wordJumps(gtx C, ed *widget.Editor) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: ed, Name: key.NameLeftArrow, Required: key.ModCtrl, Optional: key.ModShift},
			key.Filter{Focus: ed, Name: key.NameRightArrow, Required: key.ModCtrl, Optional: key.ModShift},
		)
		if !ok {
			return
		}
		k, ok := ev.(key.Event)
		if !ok || k.State != key.Press {
			continue
		}
		dir := 1
		if k.Name == key.NameLeftArrow {
			dir = -1
		}
		caret, anchor := ed.Selection()
		caret = wordJump([]rune(ed.Text()), caret, dir)
		if !k.Modifiers.Contain(key.ModShift) {
			anchor = caret
		}
		ed.SetCaret(caret, anchor)
	}
}

// wordJump returns the caret after skipping whitespace and then one word in
// direction dir (±1), like Gio's own word movement.
func wordJump(r []rune, pos, dir int) int {
	at := func(p int) rune {
		if dir < 0 {
			p--
		}
		if p < 0 || p >= len(r) {
			return ' '
		}
		return r[p]
	}
	inRange := func(p int) bool { return p > 0 && p < len(r) }
	for inRange(pos) && unicode.IsSpace(at(pos)) {
		pos += dir
	}
	pos = min(max(pos+dir, 0), len(r))
	for inRange(pos) && !unicode.IsSpace(at(pos)) {
		pos += dir
	}
	return pos
}

func (a *App) inputVisible() bool {
	switch a.m.screen {
	case model.ScreenCreateBranchInput, model.ScreenCheckoutRevisionInput, model.ScreenDeleteBranchConfirm,
		model.ScreenShelveNameInput, model.ScreenCommitMessageInput, model.ScreenFileHistorySearch,
		model.ScreenHistorySearch, model.ScreenPropertyTargetInput, model.ScreenPropertyNameInput,
		model.ScreenPropertyValueInput:
		return !a.m.commands.open
	}
	return false
}

// ---- vault password gate ----

func (a *App) layoutVaultPassword(gtx C) D {
	for {
		ev, ok := a.vaultEd.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			a.unlockVault()
		}
	}
	if a.vaultBtn.Clicked(gtx) {
		a.unlockVault()
	}
	if !gtx.Focused(&a.vaultEd) {
		gtx.Execute(key.FocusCmd{Tag: &a.vaultEd})
	}
	return layout.Center.Layout(gtx, a.narrow(440, a.panel(func(gtx C) D {
		ws := []layout.Widget{
			a.row(a.iconDisc(icoLock, a.pal.Mauve, 34), a.heading("Unlock credentials")),
			a.text("A repository uses a vault credential. Enter the hyprvault / gopass vault password.", a.pal.Subtext0),
			a.editor(&a.vaultEd, "Vault password", true),
			a.row(a.btn(&a.vaultBtn, icoKey, "Unlock", btnPrimary)),
		}
		if a.vaultErr != "" {
			ws = append(ws, a.status(a.vaultErr, ""))
		}
		if a.vaultBusy {
			ws = append(ws, a.small("resolving…", a.pal.Peach))
		}
		return a.column(gtx, ws...)
	})))
}

func (a *App) unlockVault() {
	pw := a.vaultEd.Text()
	if pw == "" || a.vaultBusy {
		a.vaultErr = "vault password cannot be empty"
		return
	}
	a.vaultBusy, a.vaultErr = true, ""
	go func() {
		configs, err := credential.Resolve(a.configs, pw)
		a.post(func() {
			a.vaultBusy = false
			if err != nil {
				a.vaultErr = err.Error()
				a.vaultEd.SetText("")
				return
			}
			a.start(svn.BuildRepos(configs))
		})
	}()
}

// ---- frame: header, body, hint bar ----

type hintDef struct{ key, desc string }

var hintKeys = map[string]string{
	"Enter": "enter", "Esc": "esc", "Space": " ", "Del": "delete", "/": "/", "i": "i", "q": "q",
	"a": "a", "n": "n", "d": "d", "m": "m", "p": "p", "h": "h", "u": "u", "r": "r", "t": "t", "M": "M", "e": "e", "s": "s",
	"Enter/m": "enter", "Enter/e": "enter", "→/Enter": "enter", "←": "left", "Backspace": "backspace",
	"Alt+↓": "alt+down", "Alt+↑": "alt+up", "PgDn": "pgdown", "PgUp": "pgup", "Home": "home", "End": "end",
}

func (a *App) frame(gtx C, title string, body layout.Widget, hints ...hintDef) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.header(gtx, title) }),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min = gtx.Constraints.Max
				return a.box(gtx, alpha(a.pal.Mantle, 0xF2), a.pal.Surface0, radius, layout.UniformInset(unit.Dp(16)), func(gtx C) D {
					gtx.Constraints.Min = gtx.Constraints.Max
					return body(gtx)
				})
			})
		}),
		layout.Rigid(func(gtx C) D { return a.hintBar(gtx, hints) }),
	)
}

func (a *App) header(gtx C, title string) D {
	r := a.m.activeRepo
	path := r.Path
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(12), Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						if a.m.screen == model.ScreenRepoSelect {
							return a.iconDisc(icoRepo, a.pal.Mauve, 30)(gtx)
						}
						return a.iconBtn(&a.backBtn, icoBack)(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx C) D {
						l := material.Label(a.th, unit.Sp(17), title)
						l.Color = a.pal.Text
						l.Font.Weight = font.SemiBold
						l.MaxLines = 1
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Flexed(1, func(gtx C) D {
						if r.Path == "" {
							return D{}
						}
						pathLabel := func(gtx C) D {
							gtx.Constraints.Max.X = gtx.Constraints.Max.X * 6 / 10
							l := material.Label(a.th, unit.Sp(12), path)
							l.Color = a.pal.Mauve
							l.Font.Weight = font.Medium
							l.MaxLines = 1
							return l.Layout(gtx)
						}
						return a.row(pathLabel, a.small("·", a.pal.Overlay0), a.selectable(&a.locSel, r.CurrentLocation, a.pal.Yellow, unit.Sp(12)), a.small("·", a.pal.Overlay0), a.mono("r"+r.CurrentRevision, a.pal.Green, unit.Sp(12)))(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if r.Path == "" {
							return D{}
						}
						return a.row(a.iconBtn(&a.infoBtn, icoInfo), a.iconBtn(&a.paletteBtn, icoBolt))(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
					layout.Rigid(a.paletteDots),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if !a.m.showInfo || r.Path == "" {
					return D{}
				}
				kv := func(k, v string) layout.Widget {
					return a.row(func(gtx C) D { gtx.Constraints.Min.X = gtx.Dp(44); return a.small(k, a.pal.Overlay1)(gtx) }, a.mono(v, a.pal.Rosewater, unit.Sp(12)))
				}
				return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(40)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(kv("Path", r.Path)), layout.Rigid(kv("URL", r.URL)), layout.Rigid(kv("Root", r.Root)), layout.Rigid(kv("Rev", r.CurrentRevision)))
				})
			}),
		)
	})
}

func (a *App) paletteDots(gtx C) D {
	cols := []color.NRGBA{a.pal.Mauve, a.pal.Yellow, a.pal.Sapphire, a.pal.Teal, a.pal.Green, a.pal.Red}
	children := make([]layout.FlexChild, 0, len(cols)*2)
	for i, c := range cols {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(5)}.Layout))
		}
		c := c
		children = append(children, layout.Rigid(func(gtx C) D {
			d := gtx.Dp(8)
			paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
			return D{Size: image.Pt(d, d)}
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (a *App) hintBar(gtx C, hints []hintDef) D {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(12), Left: unit.Dp(20), Right: unit.Dp(20)}.Layout(gtx, func(gtx C) D {
		ws := make([]layout.Widget, 0, len(hints))
		for _, h := range hints {
			h := h
			if k, ok := hintKeys[h.key]; ok {
				c := a.hintBtns[h.key+"|"+h.desc]
				if c == nil {
					c = new(widget.Clickable)
					a.hintBtns[h.key+"|"+h.desc] = c
				}
				if c.Clicked(gtx) {
					a.sendKey(k)
				}
				ws = append(ws, a.hintBtn(c, h.key, h.desc))
			} else {
				ws = append(ws, func(gtx C) D {
					return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx,
						a.row(a.mono(h.key, a.pal.Overlay1, unit.Sp(11)), a.small(h.desc, a.pal.Overlay0)))
				})
			}
		}
		return a.flow(gtx, gtx.Dp(6), ws)
	})
}

// flow lays widgets out left to right and wraps to a new row when the width
// runs out.
func (a *App) flow(gtx C, gap int, ws []layout.Widget) D {
	maxW := gtx.Constraints.Max.X
	gtx.Constraints.Min = image.Point{}
	x, y, rowH := 0, 0, 0
	for _, w := range ws {
		macro := op.Record(gtx.Ops)
		d := w(gtx)
		call := macro.Stop()
		if x > 0 && x+d.Size.X > maxW {
			x, y, rowH = 0, y+rowH+gap, 0
		}
		trans := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		trans.Pop()
		x += d.Size.X + gap
		rowH = max(rowH, d.Size.Y)
	}
	return D{Size: image.Pt(maxW, y+rowH)}
}

func (a *App) hintBtn(c *widget.Clickable, k, desc string) layout.Widget {
	return func(gtx C) D {
		bg := a.pal.Surface0
		if c.Hovered() {
			bg = a.pal.Surface1
		}
		return c.Layout(gtx, func(gtx C) D {
			return a.box(gtx, bg, color.NRGBA{}, unit.Dp(6), layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(9), Right: unit.Dp(9)}, func(gtx C) D {
				pointer.CursorPointer.Add(gtx.Ops)
				l := material.Label(a.th, unit.Sp(12), desc)
				l.Color = a.pal.Subtext1
				l.MaxLines = 1
				return a.row(a.keycap(k), l.Layout)(gtx)
			})
		})
	}
}

// ---- generic rows ----

type seg struct {
	text string
	col  color.NRGBA
	bold bool
}

type rowDef struct {
	segs   []seg
	indent int
	tint   color.NRGBA // row background tint when armed for delete, zero = none
	extra  []seg       // monospace lines drawn under the row (hunk previews, property values)
}

type color4 = color.NRGBA

// rowsList draws a cursor list. Clicking a row moves the cursor; a double
// click also presses Enter.
func (a *App) rowsList(gtx C, rows []rowDef, cursor int, setCursor func(int)) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	if len(a.rowClicks) < len(rows) {
		a.rowClicks = make([]widget.Clickable, len(rows))
	}
	for i := range rows {
		if click, ok := a.rowClicks[i].Update(gtx); ok {
			setCursor(i)
			if click.NumClicks >= 2 {
				a.sendKey("enter")
			}
		}
	}
	rowH := a.lineH(gtx) + gtx.Dp(8)
	a.keepCursorVisible(gtx, &a.list, cursor, rowH)
	ls := material.List(a.th, &a.list)
	ls.Indicator.Color = a.pal.Surface2
	ls.Track.Color = alpha(a.pal.Surface0, 0)
	return ls.Layout(gtx, len(rows), func(gtx C, i int) D {
		r := rows[i]
		return a.rowClicks[i].Layout(gtx, func(gtx C) D {
			return a.rowBox(gtx, r, rowH, i == cursor, a.rowClicks[i].Hovered())
		})
	})
}

func (a *App) keepCursorVisible(gtx C, list *widget.List, cursor, rowH int) {
	visible := max(1, gtx.Constraints.Max.Y/rowH)
	if a.lastCursorScreen != a.m.screen || a.lastCursor != cursor {
		a.lastCursorScreen, a.lastCursor = a.m.screen, cursor
		if cursor < list.Position.First {
			list.Position.First, list.Position.Offset = cursor, 0
		} else if cursor >= list.Position.First+visible {
			list.Position.First, list.Position.Offset = cursor-visible+1, 0
		}
	}
}

// rowBox draws one row: cursor bar, hover tint, segments and extra lines.
func (a *App) rowBox(gtx C, r rowDef, rowH int, selected, hovered bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{Left: unit.Dp(12 + 16*float32(r.indent)), Right: unit.Dp(10)}.Layout(gtx, func(gtx C) D {
		first := func(gtx C) D {
			gtx.Constraints.Min.Y = rowH
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, a.segChildren(r.segs)...)
		}
		if len(r.extra) == 0 {
			return first(gtx)
		}
		children := []layout.FlexChild{layout.Rigid(first)}
		for _, l := range r.extra {
			children = append(children, layout.Rigid(a.codeLine(l.text, l.col, l.bold)))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	call := macro.Stop()
	size := image.Pt(gtx.Constraints.Max.X, dims.Size.Y)
	switch {
	case r.tint.A != 0:
		paint.FillShape(gtx.Ops, alpha(r.tint, 0x30), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6)).Op(gtx.Ops))
	case selected:
		paint.FillShape(gtx.Ops, a.pal.Surface0, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6)).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, a.pal.Mauve, clip.UniformRRect(image.Rectangle{Min: image.Pt(0, gtx.Dp(7)), Max: image.Pt(gtx.Dp(3), rowH-gtx.Dp(7))}, gtx.Dp(1)).Op(gtx.Ops))
	case hovered:
		paint.FillShape(gtx.Ops, alpha(a.pal.Surface0, 0x80), clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(6)).Op(gtx.Ops))
	}
	pointer.CursorPointer.Add(gtx.Ops)
	call.Add(gtx.Ops)
	return D{Size: size}
}

func (a *App) segChildren(segs []seg) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(segs)*2)
	for i, s := range segs {
		s := s
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			l := material.Label(a.th, monoSize, s.text)
			l.Font.Typeface = monoFont
			l.MaxLines = 1
			l.Color = s.col
			if s.bold {
				l.Font.Weight = font.Bold
			}
			return l.Layout(gtx)
		}))
	}
	return children
}

// codeLine is one monospace line that never wraps.
func (a *App) codeLine(s string, c color.NRGBA, bold bool) layout.Widget {
	return func(gtx C) D {
		l := material.Label(a.th, monoSize, strings.ReplaceAll(s, "\t", "    "))
		l.Font.Typeface = monoFont
		l.MaxLines = 1
		l.Color = c
		if bold {
			l.Font.Weight = font.Bold
		}
		return l.Layout(gtx)
	}
}

func (a *App) check(selected bool) seg {
	if selected {
		return seg{icoChecked, a.pal.Yellow, false}
	}
	return seg{icoSquare, a.pal.Overlay1, false}
}

func (a *App) lines(ls ...seg) layout.Widget {
	return func(gtx C) D {
		children := make([]layout.FlexChild, 0, len(ls))
		for _, s := range ls {
			s := s
			children = append(children, layout.Rigid(func(gtx C) D {
				l := material.Label(a.th, unit.Sp(13), s.text)
				l.Color = s.col
				if s.bold {
					l.Font.Weight = font.SemiBold
				}
				return l.Layout(gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

func (a *App) listBody(intro layout.Widget, list layout.Widget, footer layout.Widget) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if intro == nil {
					return D{}
				}
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, intro)
			}),
			layout.Flexed(1, list),
			layout.Rigid(func(gtx C) D {
				if footer == nil {
					return D{}
				}
				return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, footer)
			}),
		)
	}
}

func (a *App) inputBody(label string, above, below layout.Widget) layout.Widget {
	return func(gtx C) D {
		ws := []layout.Widget{}
		if above != nil {
			ws = append(ws, above)
		}
		ws = append(ws, a.narrow(620, a.field(label, a.m.input.Editor(), a.m.input.Placeholder, true)))
		if below != nil {
			ws = append(ws, below)
		}
		return a.column(gtx, ws...)
	}
}

// textLines draws plain monospace lines in a scrolling list, coloring each
// line by the classifier (svn output, unified diffs).
func (a *App) textLines(gtx C, list *widget.List, lines []string, classify func(string) (color.NRGBA, bool)) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	ls := material.List(a.th, list)
	ls.Indicator.Color = a.pal.Surface2
	ls.Track.Color = alpha(a.pal.Surface0, 0)
	return ls.Layout(gtx, len(lines), func(gtx C, i int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		line := stripANSI(lines[i])
		c, bold := classify(line)
		return a.codeLine(line, c, bold)(gtx)
	})
}

// classifySVN colors svn status / update / log output lines by their prefix.
func (a *App) classifySVN(line string) (color.NRGBA, bool) {
	t := strings.TrimLeft(line, " ")
	switch {
	case strings.HasPrefix(t, "Index:") || strings.HasPrefix(t, "===") || strings.HasPrefix(t, "---") || strings.HasPrefix(t, "+++"):
		return a.pal.Subtext0, true
	case strings.HasPrefix(t, "A ") || strings.HasPrefix(t, "+"):
		return diffGreen, false
	case strings.HasPrefix(t, "D ") || strings.HasPrefix(t, "-"):
		return diffRed, false
	case strings.HasPrefix(t, "C ") || strings.HasPrefix(t, "!") || strings.HasPrefix(t, "Error") || strings.HasPrefix(t, "svn: E"):
		return diffRed, true
	case strings.HasPrefix(t, "U ") || strings.HasPrefix(t, "G ") || strings.HasPrefix(t, "M ") || strings.HasPrefix(t, "R ") || strings.HasPrefix(t, "~"):
		return diffYellow, false
	case strings.HasPrefix(t, "?"):
		return a.pal.Overlay1, false
	case strings.HasPrefix(t, "@@"):
		return a.pal.Sapphire, false
	}
	return a.pal.Text, false
}
