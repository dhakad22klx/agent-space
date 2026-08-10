package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const basePrompt = "agent-space>"

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	echoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	menuStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	pickedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
)

type model struct {
	choices  []string // commands offered after a leading slash
	matches  []string // choices matching what has been typed; empty = menu closed
	selected int      // highlighted row in matches
	active   string   // command awaiting its argument, "" while typing free text
	viewport viewport.Model
	input    textinput.Model
	lines    []string
	width    int

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
		input:   in,
		choices: []string{"websearch", "investigate", "recommend", "news", "ipo"},
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
	// fmt.Println("Update called with msg:", msg) // Debugging line
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
		m.width = msg.Width

		m.resizeInput()
		m.syncViewport()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// While the menu is open it owns the arrows, enter and esc.
		if len(m.matches) > 0 {
			switch msg.String() {
			case "up":
				m.selected = (m.selected - 1 + len(m.matches)) % len(m.matches)
				return m, nil
			case "down":
				m.selected = (m.selected + 1) % len(m.matches)
				return m, nil
			case "enter", "tab":
				m.pick(m.matches[m.selected])
				return m, nil
			case "esc":
				m.closeMenu()
				return m, nil
			}
		}

		switch msg.String() {
		case "esc":
			return m, tea.Quit
		case "enter":
			if quit := m.submit(); quit {
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

	// The typed text decides whether the menu is open and what it holds.
	if _, typed := msg.(tea.KeyMsg); typed {
		m.refreshMenu()
	}

	return m, tea.Batch(cmds...)
}

// refreshMenu decides whether the slash menu is open and what it lists. It is
// open while the prompt holds a bare "/word" with no argument typed yet.
func (m *model) refreshMenu() {
	typed := m.input.Value()

	// The picked command sits in the value, so editing it away drops it.
	if m.active != "" && !strings.HasPrefix(typed, m.active) {
		m.clearActive()
	}

	if m.active != "" || !strings.HasPrefix(typed, "/") || strings.Contains(typed, " ") {
		m.closeMenu()
		return
	}

	prefix := strings.ToLower(strings.TrimPrefix(typed, "/"))
	m.matches = nil
	for _, choice := range m.choices {
		if strings.HasPrefix(choice, prefix) {
			m.matches = append(m.matches, choice)
		}
	}
	if m.selected >= len(m.matches) {
		m.selected = 0
	}
	m.syncViewport()
}

func (m *model) closeMenu() {
	m.matches = nil
	m.selected = 0
	m.syncViewport()
}

// pick turns the highlighted menu row into the active command, so the prompt
// becomes ">investigate " and whatever is typed next is its argument.
func (m *model) pick(choice string) {
	m.active = choice
	m.input.SetValue(choice + " ")
	m.input.SetCursor(len(choice + " "))
	m.input.Prompt = basePrompt
	m.input.Placeholder = "what should I " + choice + "?"
	m.closeMenu()
	m.resizeInput()
}

func (m *model) clearActive() {
	m.active = ""
	m.input.Prompt = basePrompt
	m.input.Placeholder = ""
	m.resizeInput()
}

// submit runs whatever the prompt currently holds. The command name lives in
// the value itself, so the first word decides: a known choice makes this a
// command with the rest as its argument, anything else is free text.
func (m *model) submit() bool {
	typed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(m.input.Value()), "/"))
	if typed == "" {
		return false
	}

	name, arg := "", typed
	if head, rest, _ := strings.Cut(typed, " "); slices.Contains(m.choices, head) {
		name, arg = head, strings.TrimSpace(rest)
	}

	m.input.Reset()
	m.clearActive()
	return m.run(name, arg)
}

// run handles one command and reports whether the program should exit. An empty
// name means the input was free text rather than a command.
func (m *model) run(name, arg string) bool {
	label := arg
	if name != "" {
		label = strings.TrimSpace("/" + name + " " + arg)
	}
	m.appendLine(promptStyle.Render(basePrompt) + label)

	switch name {
	case "exit", "quit", "q":
		m.appendLine("Goodbye!")
		return true
	case "help":
		m.appendLine("Available commands: help, clear, exit")
	case "clear":
		m.lines = nil
		m.syncViewport()
	case "":
		PrintLoop(m)
		m.appendLine(echoStyle.Render(fmt.Sprintf("Available information: %s", arg)))
	default:
		// A known choice with no handler of its own yet.
		m.appendLine(echoStyle.Render(fmt.Sprintf("Running %s: %s", name, arg)))
	}

	return false
}

func (m *model) appendLine(line string) {
	m.lines = append(m.lines, line)
	m.syncViewport()
}

// resizeInput refits the text field, whose room depends on the prompt in front
// of it — and the prompt grows once a command is active.
func (m *model) resizeInput() {
	width := m.width - lipgloss.Width(m.input.Prompt) - 1
	if width < 1 {
		width = 1
	}
	m.input.Width = width
}

func (m *model) syncViewport() {
	if !m.ready {
		return
	}

	content := strings.Join(m.lines, "\n")

	// Grow with the output instead of always claiming the full screen, so the
	// prompt sits right under the last line until the scrollback fills up.
	// The open menu takes its rows out of the same budget.
	maxHeight := m.maxHeight - len(m.matches)
	height := lipgloss.Height(content)
	if height > maxHeight {
		height = maxHeight
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

	// help := "enter run · ↑/↓ scroll · esc or ctrl+c quit"
	// fmt.Sprintf("%s\n%s\n%s", m.viewport.View(), m.input.View(), helpStyle.Render(help))
	// if len(m.matches) > 0 {
	// 	help = "↑/↓ select · enter or tab pick · esc close menu"
	// }

	parts := []string{m.viewport.View(), m.input.View()}
	if menu := m.menuView(); menu != "" {
		parts = append(parts, menu)
	}
	// parts = append(parts, helpStyle.Render(help))

	return strings.Join(parts, "\n")
}

// menuView draws the slash menu under the prompt, highlighting the selection.
func (m model) menuView() string {
	if len(m.matches) == 0 {
		return ""
	}

	rows := make([]string, len(m.matches))
	for i, choice := range m.matches {
		if i == m.selected {
			rows[i] = pickedStyle.Render("› /" + choice)
			continue
		}
		rows[i] = menuStyle.Render("  /" + choice)
	}

	return strings.Join(rows, "\n")
}
