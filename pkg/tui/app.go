package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/liuguanyu/radio-cmd/internal/config"
	"github.com/liuguanyu/radio-cmd/pkg/radio"
)

// Styles defines all the UI styles for the application
type Styles struct {
	Title           lipgloss.Style
	Selected        lipgloss.Style
	Normal          lipgloss.Style
	Status          lipgloss.Style
	Help            lipgloss.Style
	Error           lipgloss.Style
	Loading         lipgloss.Style
}

// NewStyles creates a new set of styles
func NewStyles() *Styles {
	return &Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(1, 2),

		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 2),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 2),

		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("236")).
			Padding(1, 2),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("196")).
			Padding(2, 4),

		Loading: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(2, 4),
	}
}

// App represents the main TUI application
type App struct {
	client       *radio.Client
	player       *radio.Player
	cfg          *config.Config
	stations     []radio.Station
	cursor       int
	loading      bool
	err          error
	width        int
	height       int
	lastError    string
	needsRefresh bool
	styles       *Styles
}

// NewApp creates a new TUI application
func NewApp() *App {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	return &App{
		client: radio.NewClient(),
		player: radio.NewPlayer(),
		cfg:    cfg,
		loading: true,
		styles: NewStyles(),
	}
}

// Start launches the TUI application
func (a *App) Start() error {
	// Check for audio player dependencies
	if err := radio.CheckDependencies(); err != nil {
		return fmt.Errorf("dependency check failed:\n%w", err)
	}

	p := tea.NewProgram(a, tea.WithAltScreen())
	err := p.Start()
	return err
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return a.fetchStations
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case stationsFetchedMsg:
		a.loading = false
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch stations: %v", msg.err)
			return a, nil
		}
		a.stations = msg.stations
		a.needsRefresh = false
		return a, nil

	case tea.WindowSizeMsg:
		// Handle window resize
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	}

	return a, nil
}

// View implements tea.Model
func (a *App) View() string {
	if a.loading {
		return a.renderLoading()
	}

	if a.err != nil {
		return a.renderError()
	}

	return a.renderMainUI()
}

// Messages
type stationsFetchedMsg struct {
	stations []radio.Station
	err      error
}

// Commands
func (a *App) fetchStations() tea.Msg {
	stations, err := a.client.GetStations()
	return stationsFetchedMsg{stations: stations, err: err}
}

// Key handling
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		// Cleanup and exit
		a.player.Stop()
		return a, tea.Quit

	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}

	case "down", "j":
		if a.cursor < len(a.stations)-1 {
			a.cursor++
		}

	case "enter", " ":
		if len(a.stations) > 0 {
			station := &a.stations[a.cursor]
			if err := a.player.Play(station); err != nil {
				a.err = err
			}
		}

	case "s":
		// Stop playback
		a.player.Stop()

	case "r":
		// Refresh station list
		a.loading = true
		return a, a.fetchStations
	}

	return a, nil
}

// Rendering functions
func (a *App) renderLoading() string {
	return a.styles.Loading.Render("📻 Loading radio stations...")
}

func (a *App) renderError() string {
	errorMsg := fmt.Sprintf("Error: %v\n\nPress 'q' to quit", a.lastError)
	if a.err != nil {
		errorMsg = fmt.Sprintf("Error: %v\n\nPress 'q' to quit", a.err)
	}
	return a.styles.Error.Render(errorMsg)
}

func (a *App) renderMainUI() string {
	var b strings.Builder

	// Title
	title := a.styles.Title.Render("📻 Radio.cn CLI Player")
	b.WriteString(title + "\n\n")

	// Station list
	listHeight := min(len(a.stations), a.height-8) // Leave room for title, status, help

	// Calculate slice of stations to show
	start := 0
	if a.cursor >= listHeight-1 && listHeight > 0 {
		start = a.cursor - listHeight + 1
	}
	end := min(start+listHeight, len(a.stations))

	// Build station list display
	for i := start; i < end; i++ {
		station := a.stations[i]
		var line string

		// Add playback indicator if playing
		indicator := " "
		if a.player.IsPlaying() && a.player.Station != nil && a.player.Station.ContentID == station.ContentID {
			indicator = "▶"
		}

		if i == a.cursor {
			line = a.styles.Selected.Render(fmt.Sprintf("%s %s", indicator, station.Title))
		} else {
			line = a.styles.Normal.Render(fmt.Sprintf("  %s", station.Title))
		}

		// Add subtitle if visible
		if i == a.cursor && station.Subtitle != "" {
			subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render
			line += "\n  " + subtitleStyle(station.Subtitle)
		}

		b.WriteString(line + "\n")
	}

	// Status bar
	statusText := "Stopped"
	if a.player.IsPlaying() && a.player.Station != nil {
		statusText = fmt.Sprintf("Playing: %s | %s", a.player.Station.Title, a.player.Station.Subtitle)
	} else if a.player.GetState() == 1 { // Paused
		statusText = "Paused"
	}

	statusBar := a.styles.Status.Render(fmt.Sprintf(" Status: %s ", statusText))
	b.WriteString("\n" + statusBar + "\n")

	// Help
	help := a.styles.Help.Render(" ↑↓ Navigate | Space/Enter Play/Stop | R Refresh | Q Quit ")
	b.WriteString(help + "\n")

	// Station count
	count := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf(" Total stations: %d ", len(a.stations)))
	b.WriteString("\n" + count)

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
