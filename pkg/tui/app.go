package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/liuguanyu/radio-cmd/internal/config"
	"github.com/liuguanyu/radio-cmd/pkg/radio"
)

// Styles defines all the UI styles for the application
type Styles struct {
	Title            lipgloss.Style
	Selected         lipgloss.Style
	Normal           lipgloss.Style
	Status           lipgloss.Style
	Help             lipgloss.Style
	Error            lipgloss.Style
	Loading          lipgloss.Style
	Category         lipgloss.Style
	CategorySelected lipgloss.Style
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

		Category: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1),

		CategorySelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 1),
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

	// 分类和省份筛选状态
	categoryFilter string  // 当前选择的分类ID，"0"表示所有
	provinceFilter string  // 当前选择的省份Code（字符串），"0"表示所有
	categories     []radio.Category  // 所有分类
	provinces      []radio.Province  // 所有省份
	showCategories bool  // 是否显示分类选择
	showProvinces  bool  // 是否显示省份选择
}

// NewApp creates a new TUI application
func NewApp() *App {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	return &App{
		client:         radio.NewClient(),
		player:         radio.NewPlayer(),
		cfg:            cfg,
		loading:        true,
		styles:         NewStyles(),
		categoryFilter: cfg.DefaultCategory,
		provinceFilter: cfg.DefaultProvince, // 从配置读取为字符串
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
	return tea.Batch(
		a.fetchStations,
		a.fetchCategories,
		a.fetchProvinces,
	)
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

	case categoriesFetchedMsg:
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch categories: %v", msg.err)
			return a, nil
		}
		a.categories = msg.categories
		// 如果配置中有默认分类，选择它
		if a.categoryFilter != "0" {
			for i, cat := range a.categories {
				if cat.ID == a.categoryFilter {
					a.cursor = i
					break
				}
			}
		}
		return a, nil

	case provincesFetchedMsg:
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch provinces: %v", msg.err)
			return a, nil
		}
		a.provinces = msg.provinces
		// 如果配置中有默认省份，选择它
		if a.provinceFilter != "0" {
			// provinceFilter是字符串，需要转换为int进行比较
			if filterInt, err := strconv.Atoi(a.provinceFilter); err == nil {
				for i, prov := range a.provinces {
					if prov.Code == filterInt {
						a.cursor = i
						break
					}
				}
			}
		}
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

	// 如果正在显示分类选择
	if a.showCategories {
		return a.renderCategorySelection()
	}

	// 如果正在显示省份选择
	if a.showProvinces {
		return a.renderProvinceSelection()
	}

	// 默认：电台列表界面
	return a.renderMainUI()
}

// Messages
type stationsFetchedMsg struct {
	stations []radio.Station
	err      error
}

type categoriesFetchedMsg struct {
	categories []radio.Category
	err         error
}

type provincesFetchedMsg struct {
	provinces []radio.Province
	err        error
}

// Commands
func (a *App) fetchStations() tea.Msg {
	stations, err := a.client.GetStationsByFilter(a.categoryFilter, a.provinceFilter)
	return stationsFetchedMsg{stations: stations, err: err}
}

func (a *App) fetchCategories() tea.Msg {
	categories, err := a.client.GetCategories()
	return categoriesFetchedMsg{categories: categories, err: err}
}

func (a *App) fetchProvinces() tea.Msg {
	provinces, err := a.client.GetProvinces()
	return provincesFetchedMsg{provinces: provinces, err: err}
}

// Key handling
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 如果正在显示分类选择
	if a.showCategories {
		return a.handleCategoryKeyPress(msg)
	}

	// 如果正在显示省份选择
	if a.showProvinces {
		return a.handleProvinceKeyPress(msg)
	}

	// 默认：电台列表界面
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

	case "c":
		// Show category selection
		a.showCategories = true
		a.cursor = 0
		return a, nil

	case "p":
		// Show province selection
		a.showProvinces = true
		a.cursor = 0
		return a, nil
	}

	return a, nil
}

// handleCategoryKeyPress handles key press in category selection mode
func (a *App) handleCategoryKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.showCategories = false
		a.cursor = 0
		return a, nil

	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}

	case "down", "j":
		if a.cursor < len(a.categories)-1 {
			a.cursor++
		}

	case "enter", " ":
		// Select category
		if len(a.categories) > 0 {
			selected := a.categories[a.cursor]
			a.categoryFilter = selected.ID
			a.showCategories = false
			a.cursor = 0
			a.loading = true
			return a, a.fetchStations
		}

	case "escape":
		a.showCategories = false
		a.cursor = 0
		return a, nil
	}

	return a, nil
}

// handleProvinceKeyPress handles key press in province selection mode
func (a *App) handleProvinceKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.showProvinces = false
		a.cursor = 0
		return a, nil

	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}

	case "down", "j":
		if a.cursor < len(a.provinces)-1 {
			a.cursor++
		}

	case "enter", " ":
		// Select province
		if len(a.provinces) > 0 {
			selected := a.provinces[a.cursor]
			// 将省份Code（int）转换为字符串
			a.provinceFilter = strconv.Itoa(selected.Code)
			a.showProvinces = false
			a.cursor = 0
			a.loading = true
			return a, a.fetchStations
		}

	case "escape":
		a.showProvinces = false
		a.cursor = 0
		return a, nil
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
	help := a.styles.Help.Render(" ↑↓ Navigate | Space/Enter Play/Stop | R Refresh | C Categories | P Provinces | Q Quit ")
	b.WriteString(help + "\n")

	// Station count
	count := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf(" Total stations: %d ", len(a.stations)))
	b.WriteString("\n" + count)

	return b.String()
}

// renderCategorySelection renders the category selection interface
func (a *App) renderCategorySelection() string {
	var b strings.Builder

	// Title
	title := a.styles.Title.Render("📻 Select Category")
	b.WriteString(title + "\n\n")

	// Category list
	for i, category := range a.categories {
		var line string
		if i == a.cursor {
			line = a.styles.CategorySelected.Render(fmt.Sprintf("▶ %s", category.CategoryName))
		} else {
			line = a.styles.Category.Render(fmt.Sprintf("  %s", category.CategoryName))
		}
		b.WriteString(line + "\n")
	}

	// Help
	help := a.styles.Help.Render(" ↑↓ Navigate | Enter Select | Escape Cancel | Q Quit ")
	b.WriteString("\n" + help)

	return b.String()
}

// renderProvinceSelection renders the province selection interface
func (a *App) renderProvinceSelection() string {
	var b strings.Builder

	// Title
	title := a.styles.Title.Render("📻 Select Province")
	b.WriteString(title + "\n\n")

	// Province list
	for i, province := range a.provinces {
		var line string
		if i == a.cursor {
			line = a.styles.CategorySelected.Render(fmt.Sprintf("▶ %s", province.ProvinceName))
		} else {
			line = a.styles.Category.Render(fmt.Sprintf("  %s", province.ProvinceName))
		}
		b.WriteString(line + "\n")
	}

	// Help
	help := a.styles.Help.Render(" ↑↓ Navigate | Enter Select | Escape Cancel | Q Quit ")
	b.WriteString("\n" + help)

	return b.String()
}