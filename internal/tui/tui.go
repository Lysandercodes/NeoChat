package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	NormalMode Mode = iota
	InsertMode
	CommandMode
)

// Monochromatic styles
var (
	baseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // White text
	headerStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("15"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("15")) // Inverted for cursor
)

type model struct {
	mode     Mode
	messages []string
	input    textarea.Model
	width    int
	height   int
}

func InitialModel() model {
	ta := textarea.New()
	ta.Placeholder = "> "
	ta.Focus()
	ta.Prompt = ""
	ta.CharLimit = 4096
	ta.SetWidth(50)
	ta.SetHeight(2)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	
	// Force monochrome cursor
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	
	return model{
		mode:     NormalMode,
		messages: []string{"Trinity 21:31\nAre you awake?", "Neo 21:32\nYeah"},
		input:    ta,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Emergency exit
		if msg.Type == tea.KeyCtrlBackslash {
			return m, tea.Quit
		}

		switch m.mode {
		case NormalMode:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "i", "enter":
				m.mode = InsertMode
				m.input.Focus()
				return m, textarea.Blink
			case ":":
				m.mode = CommandMode
				m.input.Placeholder = ":"
				m.input.Focus()
				return m, textarea.Blink
			}

		case CommandMode:
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = NormalMode
				m.input.Blur()
				m.input.Placeholder = "> "
				m.input.Reset()
			case tea.KeyEnter:
				cmdStr := strings.TrimSpace(m.input.Value())
				if strings.HasPrefix(cmdStr, "push_update") {
					m.messages = append(m.messages, "SYSTEM Now\nPushing OTA Update to Trinity...")
					// Call IPC client here
				} else if strings.HasPrefix(cmdStr, "exec_remote") {
					m.messages = append(m.messages, "SYSTEM Now\nExecuting remote script on Trinity's machine...")
				} else if cmdStr == "logs trinity" {
					m.messages = append(m.messages, "SYSTEM Now\nStreaming Trinity Telemetry Logs...")
				}
				m.mode = NormalMode
				m.input.Blur()
				m.input.Placeholder = "> "
				m.input.Reset()
			default:
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}

		case InsertMode:
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = NormalMode
				m.input.Blur()
			case tea.KeyEnter:
				if !msg.Alt { // Send message on enter (without Alt/Shift)
					val := m.input.Value()
					if strings.TrimSpace(val) != "" {
						m.messages = append(m.messages, "Neo Now\n"+val)
						m.input.Reset()
					}
					m.mode = NormalMode
					m.input.Blur()
				} else {
					m.input, cmd = m.input.Update(msg)
					cmds = append(cmds, cmd)
				}
			default:
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder

	// Header
	header := fmt.Sprintf("Trinity ● online %*s SYNCED ✓", m.width-36, "")
	b.WriteString(headerStyle.Render(header) + "\n\n")

	// Messages
	for _, msg := range m.messages {
		b.WriteString(baseStyle.Render(msg) + "\n\n")
	}

	// Input area
	if m.mode == InsertMode {
		b.WriteString("\nINSERT | ")
		b.WriteString(m.input.View())
	} else if m.mode == CommandMode {
		b.WriteString("\nCOMMAND | :")
		b.WriteString(m.input.View())
	} else if m.mode == NormalMode {
		b.WriteString("\nNORMAL | > _")
	}

	return baseStyle.Render(b.String())
}

func StartTUI() error {
	p := tea.NewProgram(InitialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
