// Package tea is the tiny slice of the Bubble Tea API that the svn-tui state
// machine uses, so that code runs unchanged under Gio. Commands run in
// goroutines and their messages are posted back to the UI loop by the caller.
package tea

import (
	"os/exec"
	"path/filepath"
	"strings"
)

type (
	Msg   = any
	Model = any
	Cmd   func() Msg
)

type KeyType int

const (
	KeyRunes KeyType = iota
	KeyEsc
	KeyEnter
	KeyUp
	KeyDown
	KeyDelete
)

// KeyMsg is a key press. Name, when set, is the exact Bubble Tea key string
// ("ctrl+p", "pgup", " "); the GUI fills it in directly.
type KeyMsg struct {
	Type  KeyType
	Runes []rune
	Alt   bool
	Name  string
}

func (k KeyMsg) String() string {
	if k.Name != "" {
		return k.Name
	}
	base := string(k.Runes)
	switch k.Type {
	case KeyEsc:
		base = "esc"
	case KeyEnter:
		base = "enter"
	case KeyUp:
		base = "up"
	case KeyDown:
		base = "down"
	case KeyDelete:
		base = "delete"
	}
	if k.Alt {
		return "alt+" + base
	}
	return base
}

// Key builds a KeyMsg from a Bubble Tea key string.
func Key(name string) KeyMsg {
	if len([]rune(name)) == 1 {
		return KeyMsg{Type: KeyRunes, Runes: []rune(name), Name: name}
	}
	return KeyMsg{Name: name}
}

type WindowSizeMsg struct{ Width, Height int }

type MouseEventType int

const (
	MouseWheelUp MouseEventType = iota + 1
	MouseWheelDown
)

type MouseMsg struct{ Type MouseEventType }

type QuitMsg struct{}

func Quit() Msg { return QuitMsg{} }

// BatchMsg carries the commands of a Batch; the runner starts each of them.
type BatchMsg []Cmd

func Batch(cmds ...Cmd) Cmd {
	var live BatchMsg
	for _, c := range cmds {
		if c != nil {
			live = append(live, c)
		}
	}
	if len(live) == 0 {
		return nil
	}
	return func() Msg { return live }
}

// TerminalWrapper turns a terminal program into a command that opens it in a
// terminal emulator window. The GUI sets it; without one the tool runs bare.
var TerminalWrapper func(*exec.Cmd) *exec.Cmd

// TerminalTools are executables that need a TTY (the merger TUI).
var TerminalTools = map[string]bool{"merger": true}

// ExecProcess runs the command to completion and returns fn(err) as the
// message, like Bubble Tea's, only without a terminal to hand over.
func ExecProcess(c *exec.Cmd, fn func(error) Msg) Cmd {
	return func() Msg {
		if TerminalWrapper != nil && TerminalTools[strings.TrimSuffix(filepath.Base(c.Path), ".exe")] {
			c = TerminalWrapper(c)
		}
		return fn(c.Run())
	}
}
