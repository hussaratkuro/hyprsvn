// Package viewport mirrors the bubbles viewport surface svn-tui uses: a block
// of lines with a scroll offset. The GUI draws the lines itself.
package viewport

import (
	"strings"

	"github.com/hussaratkuro/hyprsvn/internal/tea"
)

type Model struct {
	Width, Height int
	YOffset       int
	lines         []string
}

func New(width, height int) Model { return Model{Width: width, Height: height} }

func (m *Model) SetContent(s string) {
	m.lines = strings.Split(s, "\n")
	m.SetYOffset(m.YOffset)
}

func (m Model) Lines() []string       { return m.lines }
func (m Model) TotalLineCount() int   { return len(m.lines) }
func (m Model) VisibleLineCount() int { return min(max(1, m.Height), len(m.lines)) }
func (m *Model) GotoTop()             { m.YOffset = 0 }
func (m *Model) GotoBottom()          { m.YOffset = m.maxOffset() }
func (m Model) AtBottom() bool        { return m.YOffset >= m.maxOffset() }
func (m *Model) SetYOffset(n int)     { m.YOffset = max(0, min(n, m.maxOffset())) }
func (m Model) maxOffset() int        { return max(0, len(m.lines)-max(1, m.Height)) }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.SetYOffset(m.YOffset - 1)
		case "down", "j":
			m.SetYOffset(m.YOffset + 1)
		case "pgup":
			m.SetYOffset(m.YOffset - max(1, m.Height))
		case "pgdown", " ":
			m.SetYOffset(m.YOffset + max(1, m.Height))
		case "home":
			m.GotoTop()
		case "end":
			m.GotoBottom()
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp {
			m.SetYOffset(m.YOffset - 3)
		} else {
			m.SetYOffset(m.YOffset + 3)
		}
	}
	return m, nil
}

func (m Model) View() string {
	end := min(len(m.lines), m.YOffset+max(1, m.Height))
	if m.YOffset >= end {
		return ""
	}
	return strings.Join(m.lines[m.YOffset:end], "\n")
}
