package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	echoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type model struct {
	viewport  viewport.Model
	input     textinput.Model
	lines     []string
	maxHeight int // rows the scrollback may use once it fills the screen
	ready     bool
}

func NewModel() model {
	in := textinput.New()
	in.Prompt = "agent-space> "
	in.PromptStyle = promptStyle
	in.CharLimit = 512
	in.Focus()

	return model{
		input: in,
		lines: []string{
			"Welcome to the Agent-Space! Your personal AI assistant",
			helpStyle.Render("Enter your prompt or type a command, help for the command list."),
		},
	}
}

func (m model) Init() tea.Cmd {
	// The prompt is focused from the start, so blink its cursor.
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Reserve two lines for the prompt and the help footer.
		m.maxHeight = msg.Height - 2
		if m.maxHeight < 1 {
			m.maxHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, m.maxHeight)
			m.ready = true
		}
		m.viewport.Width = msg.Width

		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1
		m.syncViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			input := strings.TrimSpace(m.input.Value())
			m.input.Reset()
			if input == "" {
				return m, nil
			}
			if quit := m.run(input); quit {
				return m, tea.Quit
			}
			return m, nil
		case "up", "down", "pgup", "pgdown":
			// The prompt ignores these, so let them scroll the scrollback.
			if m.ready {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
			return m, nil
		}
	}

	// Everything else belongs to the prompt, which is always focused.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// run handles one command and reports whether the program should exit.
func (m *model) run(input string) bool {
	m.appendLine(promptStyle.Render("agent-space> ") + input)

	switch input {
	case "exit", "quit", "q":
		m.appendLine("Goodbye!")
		return true
	case "help":
		m.appendLine("Available commands: help, clear, exit")
	case "clear":
		m.lines = nil
		m.syncViewport()
	default:
		PrintLoop(m)
		m.appendLine(echoStyle.Render(fmt.Sprintf("Available information: %s", input)))
	}

	return false
}

func (m *model) appendLine(line string) {
	m.lines = append(m.lines, line)
	m.syncViewport()
}

func (m *model) syncViewport() {
	if !m.ready {
		return
	}

	content := strings.Join(m.lines, "\n")

	// Grow with the output instead of always claiming the full screen, so the
	// prompt sits right under the last line until the scrollback fills up.
	height := lipgloss.Height(content)
	if height > m.maxHeight {
		height = m.maxHeight
	}
	if height < 1 {
		height = 1
	}
	m.viewport.Height = height

	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m model) View() string {
	if !m.ready {
		return titleStyle.Render("Agent-Space") + "\n"
	}

	help := "enter run · ↑/↓ scroll · esc or ctrl+c quit"

	return fmt.Sprintf("%s\n%s\n%s", m.viewport.View(), m.input.View(), helpStyle.Render(help))
}
