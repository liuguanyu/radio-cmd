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
	Province         lipgloss.Style
	ProvinceSelected lipgloss.Style
	Panel            lipgloss.Style
	PanelTitle       lipgloss.Style
	Subtitle         lipgloss.Style
	Muted            lipgloss.Style
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
			Padding(0, 1),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1),

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

		Province: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(0, 1),

		ProvinceSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 1),

		Panel: lipgloss.NewStyle().
			Padding(0, 1),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81")),

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}
}

// App represents the main TUI application
type App struct {
	client        *radio.Client
	player        *radio.Player
	cfg           *config.Config
	stations      []radio.Station
	cursor        int
	loading       bool
	inlineLoading bool
	err           error
	width         int
	height        int
	lastError     string
	needsRefresh  bool
	styles        *Styles

	provinceFilter string
	provinces      []radio.Province
	provinceCursor int
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
		provinceFilter: cfg.DefaultProvince,
	}
}

// Start launches the TUI application
func (a *App) Start() error {
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
		a.inlineLoading = false
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch stations: %v", msg.err)
			return a, nil
		}
		a.err = nil
		a.stations = msg.stations
		a.needsRefresh = false
		if len(a.stations) == 0 {
			a.cursor = 0
		} else if a.cursor >= len(a.stations) {
			a.cursor = len(a.stations) - 1
		}
		return a, nil

	case provincesFetchedMsg:
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch provinces: %v", msg.err)
			return a, nil
		}
		a.provinces = append([]radio.Province{{Code: 0, ProvinceName: "全部"}}, msg.provinces...)
		a.syncProvinceCursor()
		return a, nil

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	}

	return a, nil
}

// View implements tea.Model
func (a *App) View() string {
	if a.loading && len(a.stations) == 0 {
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

type provincesFetchedMsg struct {
	provinces []radio.Province
	err       error
}

// Commands
func (a *App) fetchStations() tea.Msg {
	stations, err := a.client.GetStationsByFilter("0", a.provinceFilter)
	return stationsFetchedMsg{stations: stations, err: err}
}

func (a *App) fetchProvinces() tea.Msg {
	provinces, err := a.client.GetProvinces()
	return provincesFetchedMsg{provinces: provinces, err: err}
}

// Key handling
func (a *App) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
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

	case "left", "a":
		if a.provinceCursor > 0 {
			a.provinceCursor--
			return a, a.applyProvinceSelection()
		}

	case "right", "d":
		if a.provinceCursor < len(a.provinces)-1 {
			a.provinceCursor++
			return a, a.applyProvinceSelection()
		}

	case "enter", " ":
		if len(a.stations) > 0 {
			station := &a.stations[a.cursor]
			if err := a.player.Play(station); err != nil {
				a.err = err
				a.lastError = err.Error()
			}
		}

	case "x":
		a.player.Stop()

	case "r":
		a.loading = true
		return a, a.fetchStations
	}

	return a, nil
}

func (a *App) applyProvinceSelection() tea.Cmd {
	if len(a.provinces) == 0 || a.provinceCursor >= len(a.provinces) {
		return nil
	}

	selected := a.provinces[a.provinceCursor]
	a.provinceFilter = strconv.Itoa(selected.Code)
	a.cursor = 0
	a.loading = true
	a.inlineLoading = true
	a.err = nil
	a.lastError = ""
	return a.fetchStations
}

func (a *App) syncProvinceCursor() {
	if len(a.provinces) == 0 {
		a.provinceCursor = 0
		return
	}

	filterInt, err := strconv.Atoi(a.provinceFilter)
	if err != nil {
		filterInt = 0
	}

	for i, prov := range a.provinces {
		if prov.Code == filterInt {
			a.provinceCursor = i
			return
		}
	}

	a.provinceCursor = 0
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

	leftWidth := 26
	rightWidth := 58
	if a.width > 0 {
		innerWidth := max(48, a.width-2)
		leftWidth = max(24, min(32, innerWidth/3))
		rightWidth = max(36, innerWidth-leftWidth-2)
	}
	contentHeight := max(8, a.height-3)

	provincePanel := a.renderProvincePanel(leftWidth, contentHeight)
	stationPanel := a.renderStationPanel(rightWidth, contentHeight)

	// 使用更简单的边框拼接方式，不再使用 Panel 的整体边框
	borderStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("238"))
	leftWithBorder := borderStyle.Render(provincePanel)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, leftWithBorder, stationPanel)
	b.WriteString(mainContent)

	statusText := "未播放"
	if a.player.IsPlaying() && a.player.Station != nil {
		statusText = fmt.Sprintf("播放中：%s", a.player.Station.Title)
		if a.player.Station.Subtitle != "" {
			statusText = fmt.Sprintf("%s / %s", statusText, a.player.Station.Subtitle)
		}
	} else if a.player.GetState() == 1 {
		statusText = "已暂停"
	}

	statusWidth := leftWidth + rightWidth + 2
	helpContent := fmt.Sprintf(
		"状态：%s | 省份：%s | 电台：%d | W/S 选台 | A/D 切省 | Enter/Space 播放 | X 停止 | R 刷新 | Q 退出",
		statusText,
		a.currentProvinceName(),
		len(a.stations),
	)
	help := a.styles.Help.Width(statusWidth).MaxWidth(statusWidth).Render(truncateRunes(helpContent, max(10, statusWidth)))
	b.WriteString("\n" + help)

	return b.String()
}

func (a *App) renderProvincePanel(width, height int) string {
	var lines []string
	title := lipgloss.NewStyle().Width(width - 4).Render(a.styles.PanelTitle.Render("省份"))
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Width(width - 4).Render(strings.Repeat("─", max(1, width-4)))
	lines = append(lines, title, separator)

	visibleHeight := max(1, height-5)
	start, end := windowRange(len(a.provinces), visibleHeight, a.provinceCursor)

	for i := start; i < end; i++ {
		province := a.provinces[i]
		line := fmt.Sprintf("  %s", province.ProvinceName)
		if i == a.provinceCursor {
			line = a.styles.ProvinceSelected.Width(width - 4).Render("▶ " + province.ProvinceName)
		} else {
			line = a.styles.Province.Width(width - 4).Render(line)
		}
		lines = append(lines, line)
	}

	for len(lines) < visibleHeight+2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return a.styles.Panel.Width(width).Height(height).Render(content)
}

func (a *App) renderStationPanel(width, height int) string {
	var lines []string
	title := lipgloss.NewStyle().Width(width - 4).Render(a.styles.PanelTitle.Render("电台列表"))
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Width(width - 4).Render(strings.Repeat("─", max(1, width-4)))
	lines = append(lines, title, separator)

	if len(a.stations) == 0 {
		emptyText := "当前省份暂无电台"
		if a.inlineLoading {
			emptyText = "正在切换省份..."
		}
		lines = append(lines, a.styles.Muted.Width(width-4).Render(emptyText))
		content := strings.Join(lines, "\n")
		return a.styles.Panel.Width(width).Height(height).Render(content)
	}

	visibleHeight := max(1, height-5)
	start, end := windowRange(len(a.stations), visibleHeight, a.cursor)

	for i := start; i < end; i++ {
		station := a.stations[i]
		indicator := "  "
		if a.player.IsPlaying() && a.player.Station != nil && a.player.Station.ContentID == station.ContentID {
			indicator = "▶ "
		}

		title := indicator + station.Title
		if i == a.cursor {
			lines = append(lines, a.styles.Selected.Width(width-4).Render(title))
			if station.Subtitle != "" {
				lines = append(lines, a.styles.Subtitle.Width(width-4).Render("  "+station.Subtitle))
			}
		} else {
			lines = append(lines, a.styles.Normal.Width(width-4).Render(title))
		}
	}

	for len(lines) < visibleHeight+2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	if a.inlineLoading {
		content += "\n" + a.styles.Muted.Width(width-4).Render("正在切换省份...")
	}
	return a.styles.Panel.Width(width).Height(height).Render(content)
}

func (a *App) currentProvinceName() string {
	if len(a.provinces) == 0 || a.provinceCursor >= len(a.provinces) {
		return "全部"
	}
	return a.provinces[a.provinceCursor].ProvinceName
}

func windowRange(total, visible, cursor int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if total <= visible {
		return 0, total
	}

	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > total {
		end = total
		start = end - visible
	}
	return start, end
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}
