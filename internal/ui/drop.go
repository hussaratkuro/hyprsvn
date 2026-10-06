package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// Dragging selected commit files onto a hyprcommander pane copies them there
// with their relative directories. Wayland gives no cross-application drag,
// so on release hyprsvn asks Hyprland which window is under the cursor and
// hands the file list to hyprcommander over its unix socket.

type dragState struct {
	tag       int
	pressed   bool
	pressAt   f32.Point
	pressRow  int
	active    bool
	pos       f32.Point // pointer position inside the list area
	count     int
	hoverRow  int
	status    string
	statusErr bool
}

// dropRequest is the message hyprcommander receives.
type dropRequest struct {
	X     int      `json:"x"`
	Y     int      `json:"y"`
	Root  string   `json:"root"`
	Files []string `json:"files"`
}

func dropSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "hyprcommander.sock")
}

// dragRows is rowsList with drag support for the commit screen. The drag
// area is registered AFTER the list so it sits on top of the list's own
// scroll area (Gio delivers pointer events to the topmost area only);
// pointer.PassOp lets wheel events through to the list underneath.
func (a *App) dragRows(gtx C, rows []rowDef, cursor int, setCursor func(int)) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	d := &a.drag
	rowH := a.lineH(gtx) + gtx.Dp(8)
	rowAt := func(p f32.Point) int {
		y := int(p.Y) + a.list.Position.First*rowH + a.list.Position.Offset
		i := y / rowH
		if p.Y < 0 || i < 0 || i >= len(rows) {
			return -1
		}
		return i
	}
	scrollbarW := gtx.Dp(16)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &d.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Move | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Move:
			d.hoverRow = rowAt(pe.Position)
		case pointer.Leave:
			d.hoverRow = -1
		case pointer.Press:
			if pe.Buttons&pointer.ButtonPrimary == 0 || int(pe.Position.X) > gtx.Constraints.Max.X-scrollbarW {
				continue
			}
			d.pressed, d.pressAt, d.pressRow, d.active = true, pe.Position, rowAt(pe.Position), false
			dragLog("press row=%d", d.pressRow)
		case pointer.Drag:
			if !d.pressed {
				continue
			}
			if !d.active && d.pressRow >= 0 {
				dx, dy := pe.Position.X-d.pressAt.X, pe.Position.Y-d.pressAt.Y
				if dx*dx+dy*dy > float32(gtx.Dp(8)*gtx.Dp(8)) {
					d.active = true
					d.count = len(a.dragFiles())
					d.status = ""
					dragLog("drag started with %d file(s)", d.count)
				}
			}
			d.pos = pe.Position
		case pointer.Cancel:
			if d.pressed {
				dragLog("pointer cancel (left the window without an implicit grab); active=%v", d.active)
			}
			d.pressed, d.active = false, false
		case pointer.Release:
			if !d.pressed {
				continue
			}
			d.pressed = false
			if d.active {
				d.active = false
				dragLog("release after drag at %v", pe.Position)
				a.finishDrop()
				continue
			}
			if d.pressRow >= 0 {
				// One click selects the row and toggles its checkbox, like
				// Space does (a directory row toggles its whole group).
				setCursor(d.pressRow)
				a.sendKey(" ")
			}
		}
	}
	a.keepCursorVisible(gtx, &a.list, cursor, rowH)
	ls := material.List(a.th, &a.list)
	ls.Indicator.Color = a.pal.Surface2
	ls.Track.Color = alpha(a.pal.Surface0, 0)
	dims := ls.Layout(gtx, len(rows), func(gtx C, i int) D {
		return a.rowBox(gtx, rows[i], rowH, i == cursor, i == d.hoverRow && !d.active)
	})
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &d.tag)
	pointer.CursorPointer.Add(gtx.Ops)
	pass.Pop()
	area.Pop()
	if d.active {
		a.layoutDragGhost(gtx)
	}
	return dims
}

// dragLog appends drag diagnostics to $XDG_RUNTIME_DIR/hyprsvn-drag.log.
func dragLog(format string, args ...any) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.OpenFile(filepath.Join(dir, "hyprsvn-drag.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("15:04:05.000 ")+format+"\n", args...)
}

// dragFiles are the selected commit files, or the file under the pointer.
func (a *App) dragFiles() []model.CommitItem {
	var files []model.CommitItem
	for _, it := range selectedCommitItems(a.m.commitItems) {
		if !it.IsDir {
			files = append(files, it)
		}
	}
	if len(files) == 0 && a.drag.pressRow >= 0 && a.drag.pressRow < len(a.m.commitItems) {
		if it := a.m.commitItems[a.drag.pressRow]; !it.IsDir {
			files = append(files, it)
		}
	}
	return files
}

// layoutDragGhost draws the "N files" badge following the pointer.
func (a *App) layoutDragGhost(gtx C) D {
	d := &a.drag
	if !d.active {
		return D{}
	}
	gtx.Constraints.Min = image.Point{}
	pos := image.Pt(int(d.pos.X), int(d.pos.Y))
	trans := op.Offset(pos.Add(image.Pt(gtx.Dp(14), gtx.Dp(10)))).Push(gtx.Ops)
	defer trans.Pop()
	label := fmt.Sprintf("%d file(s) → hyprcommander pane", d.count)
	return a.box(gtx, a.pal.Mauve, alpha(a.pal.Text, 0x40), unit.Dp(8), layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(10), Right: unit.Dp(10)}, func(gtx C) D {
		l := material.Label(a.th, unit.Sp(12), label)
		l.Color = a.pal.OnAccent
		l.MaxLines = 1
		return l.Layout(gtx)
	})
}

// finishDrop finds the hyprcommander window under the cursor and sends it
// the files.
func (a *App) finishDrop() {
	files := a.dragFiles()
	if len(files) == 0 {
		return
	}
	root := a.m.activeRepo.Path
	rels := make([]string, 0, len(files))
	for _, f := range files {
		rels = append(rels, f.Path)
	}
	go func() {
		x, y, err := cursorPosition()
		if err != nil {
			dragLog("hyprctl cursorpos failed: %v", err)
			a.post(func() { a.drag.status, a.drag.statusErr = "Drop failed: "+err.Error(), true })
			return
		}
		if !windowUnderCursor("hyprcommander", x, y) {
			dragLog("no hyprcommander window under %d,%d", x, y)
			a.post(func() { a.drag.status, a.drag.statusErr = "Drop the files onto a hyprcommander window", true })
			return
		}
		dragLog("hyprcommander under %d,%d, sending %d file(s)", x, y, len(rels))
		data, _ := json.Marshal(dropRequest{X: x, Y: y, Root: root, Files: rels})
		conn, err := net.DialTimeout("unix", dropSocketPath(), 2*time.Second)
		if err != nil {
			dragLog("socket %s: %v", dropSocketPath(), err)
			a.post(func() { a.drag.status, a.drag.statusErr = "hyprcommander is not listening: "+err.Error(), true })
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
		_, _ = conn.Write(append(data, '\n'))
		reply := make([]byte, 4096)
		n, _ := conn.Read(reply)
		msg := string(reply[:n])
		dragLog("reply: %q", msg)
		a.post(func() {
			a.drag.status, a.drag.statusErr = msg, len(msg) > 0 && msg[0] == '!'
			if msg == "" {
				a.drag.status = fmt.Sprintf("Sent %d file(s) to hyprcommander", len(rels))
			}
		})
	}()
}

func cursorPosition() (int, int, error) {
	out, err := exec.Command("hyprctl", "cursorpos", "-j").Output()
	if err != nil {
		return 0, 0, err
	}
	var pos struct{ X, Y int }
	if err := json.Unmarshal(out, &pos); err != nil {
		return 0, 0, err
	}
	return pos.X, pos.Y, nil
}

// windowUnderCursor reports whether a Hyprland client of the class covers
// the point.
func windowUnderCursor(class string, x, y int) bool {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return false
	}
	var clients []struct {
		Class string `json:"class"`
		At    [2]int `json:"at"`
		Size  [2]int `json:"size"`
	}
	if err := json.Unmarshal(out, &clients); err != nil {
		return false
	}
	for _, c := range clients {
		if c.Class == class && x >= c.At[0] && x < c.At[0]+c.Size[0] && y >= c.At[1] && y < c.At[1]+c.Size[1] {
			return true
		}
	}
	return false
}

var _ widget.Clickable
