package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220")).
			MarginTop(1).
			MarginBottom(1)

	quitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

type Model struct {
	Status           string
	Version          string
	LocalDestination string
	PublicURL        string
	ServerURL        string
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	s := titleStyle.Render("iskndr") + "\n\n"

	s += fmt.Sprintf("%s %s\n",
		labelStyle.Render("Session Status"),
		successStyle.Render(m.Status))
	s += fmt.Sprintf("%s %s\n",
		labelStyle.Render("Version       "),
		valueStyle.Render(m.Version))
	s += fmt.Sprintf("%s %s\n",
		labelStyle.Render("Tunnel Server "),
		valueStyle.Render(m.ServerURL))

	s += "\n" + subtitleStyle.Render("Forwarding") + "\n"
	s += fmt.Sprintf("%s -> %s\n",
		urlStyle.Render(m.PublicURL),
		labelStyle.Render(m.LocalDestination))

	s += "\n" + quitStyle.Render("Press Ctrl+C twice to quit")

	return s
}

func InitUi(destinationAddress, serverUrl, subdomain string) *tea.Program {
	model := NewModel()
	model.Status = "online"
	model.Version = "0.1.0"
	model.LocalDestination = destinationAddress
	model.PublicURL = subdomain
	model.ServerURL = serverUrl

	program := tea.NewProgram(model)
	go func() {
		if _, err := program.Run(); err != nil {
			fmt.Println("UI error", err)
		}
	}()
	return program
}
