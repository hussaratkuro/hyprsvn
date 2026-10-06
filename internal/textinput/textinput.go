// Package textinput mirrors the bubbles textinput surface svn-tui uses, backed
// by a Gio editor that the GUI lays out. Typing goes straight to the editor,
// so Update is a no-op.
package textinput

import (
	"gioui.org/widget"

	"github.com/hussaratkuro/hyprsvn/internal/tea"
)

type Model struct {
	Placeholder string
	CharLimit   int
	Width       int
	ed          *widget.Editor
	focused     bool
}

func New() Model {
	return Model{ed: &widget.Editor{SingleLine: true, Submit: true}}
}

func (m Model) Value() string                   { return m.ed.Text() }
func (m *Model) SetValue(s string)              { m.ed.SetText(s) }
func (m *Model) Reset()                         { m.ed.SetText("") }
func (m *Model) Focus()                         { m.focused = true }
func (m *Model) Blur()                          { m.focused = false }
func (m Model) Focused() bool                   { return m.focused }
func (m *Model) CursorEnd()                     { n := m.ed.Len(); m.ed.SetCaret(n, n) }
func (m Model) Update(tea.Msg) (Model, tea.Cmd) { return m, nil }
func (m Model) View() string                    { return m.ed.Text() }
func (m Model) Editor() *widget.Editor          { return m.ed }
