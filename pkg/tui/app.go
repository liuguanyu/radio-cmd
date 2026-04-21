package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
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
			Padding(0),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81")),

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),

		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}
}

// ViewMode represents the current view mode of the application
type ViewMode int

const (
	MainView ViewMode = iota
	ScheduleListView
	ScheduleFormView
)

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

	// Schedule related fields
	currentView      ViewMode
	schedules        []config.Schedule
	scheduleCursor   int
	formSchedule     config.Schedule
	timeInput        string
	selectedStation  *radio.Station
	stationSelecting bool
	recurring        bool
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
		styles:         NewStyles(),
		provinceFilter: cfg.DefaultProvince,
		currentView:    MainView,
		schedules:      cfg.Schedules,
		timeInput:      time.Now().Format("15:04"),
		recurring:      true,
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
		a.provinces = msg.provinces
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
	if a.err != nil {
		return a.renderError()
	}

	switch a.currentView {
	case MainView:
		return a.renderMainUI()
	case ScheduleListView:
		return a.renderScheduleList()
	case ScheduleFormView:
		return a.renderScheduleForm()
	default:
		return a.renderMainUI()
	}
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
	switch a.currentView {
	case MainView:
		return a.handleMainViewKeys(msg)
	case ScheduleListView:
		return a.handleScheduleListKeys(msg)
	case ScheduleFormView:
		return a.handleScheduleFormKeys(msg)
	default:
		return a.handleMainViewKeys(msg)
	}
}

func (a *App) handleMainViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.player.Stop()
		return a, tea.Quit

	case "q":
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

	case "s":
		// Switch to schedule list view
		a.currentView = ScheduleListView
		return a, nil
	}

	return a, nil
}

func (a *App) handleScheduleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.player.Stop()
		return a, tea.Quit

	case "q":
		// Return to main view
		a.currentView = MainView
		return a, nil

	case "up", "k":
		if len(a.schedules) > 0 && a.scheduleCursor > 0 {
			a.scheduleCursor--
		}

	case "down", "j":
		if len(a.schedules) > 0 && a.scheduleCursor < len(a.schedules)-1 {
			a.scheduleCursor++
		}

	case "enter":
		if len(a.schedules) > 0 {
			// Edit selected schedule
			a.formSchedule = a.schedules[a.scheduleCursor]
			a.timeInput = a.formSchedule.PlayTime
			a.recurring = a.formSchedule.Recurring
			a.formSchedule.Enabled = true
			a.currentView = ScheduleFormView
			return a, nil
		}

	case "n", "N":
		// Create new schedule
		a.formSchedule = config.Schedule{
			ID:        uuid.New().String(),
			CreatedAt: time.Now().Unix(),
			Enabled:   true,
		}
		a.timeInput = time.Now().Format("15:04")
		a.recurring = true
		a.selectedStation = nil
		a.currentView = ScheduleFormView
		return a, nil

	case "del", "delete":
		if len(a.schedules) > 0 {
			// Remove schedule
			a.schedules = append(a.schedules[:a.scheduleCursor], a.schedules[a.scheduleCursor+1:]...)
			if a.scheduleCursor >= len(a.schedules) && len(a.schedules) > 0 {
				a.scheduleCursor = len(a.schedules) - 1
			}
			// Update config
			a.cfg.Schedules = a.schedules
			a.saveConfig()
		}
	}

	return a, nil
}

func (a *App) handleScheduleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		a.player.Stop()
		return a, tea.Quit

	case "ctrl+s":
		// Save schedule
		if a.selectedStation != nil {
			a.formSchedule.StationID = a.selectedStation.ContentID
			a.formSchedule.StationName = a.selectedStation.Title
			a.formSchedule.PlayTime = a.timeInput
			a.formSchedule.Recurring = a.recurring

			// Check if it's a new schedule or editing existing
			if a.formSchedule.ID == "" {
				// New schedule
				a.formSchedule.ID = uuid.New().String()
				a.formSchedule.CreatedAt = time.Now().Unix()
				a.schedules = append(a.schedules, a.formSchedule)
			} else {
				// Update existing schedule
				for i, s := range a.schedules {
					if s.ID == a.formSchedule.ID {
						a.schedules[i] = a.formSchedule
						break
					}
				}
			}

			// Update config
			a.cfg.Schedules = a.schedules
			a.saveConfig()

			// Return to schedule list
			a.currentView = ScheduleListView
		}
		return a, nil

	case "esc":
		// Return to schedule list without saving
		a.currentView = ScheduleListView
		return a, nil
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

func (a *App) renderError() string {
	errorMsg := fmt.Sprintf("Error: %v\n\nPress 'q' to quit", a.lastError)
	if a.err != nil {
		errorMsg = fmt.Sprintf("Error: %v\n\nPress 'q' to quit", a.err)
	}
	return a.styles.Error.Render(errorMsg)
}

func (a *App) renderScheduleList() string {
	var b strings.Builder

	// Header
	b.WriteString(a.styles.Title.Render("播放计划列表") + "\n\n")

	if len(a.schedules) == 0 {
		b.WriteString(a.styles.Muted.Render("暂无播放计划") + "\n\n")
	} else {
		// Column headers
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-20s %-20s %-10s %-10s", "时间", "电台", "类型", "状态")) + "\n")
		b.WriteString(strings.Repeat("─", 70) + "\n")

		// Schedule list
		for i, schedule := range a.schedules {
			var line string
			if schedule.Recurring {
				line = fmt.Sprintf("%-20s %-20s %-10s ", schedule.PlayTime, schedule.StationName, "每日")
			} else {
				line = fmt.Sprintf("%-20s %-20s %-10s ", schedule.PlayTime, schedule.StationName, "单次")
			}

			if schedule.Enabled {
				line += "启用"
			} else {
				line += "禁用"
			}

			if i == a.scheduleCursor {
				b.WriteString(a.styles.Selected.Render(line) + "\n")
			} else {
				b.WriteString(a.styles.Normal.Render(line) + "\n")
			}
		}
	}

	// Help text
	helpText := "↑/↓ 选择 | Enter 编辑 | Del 删除 | N 新建 | Q 返回主界面"
	b.WriteString("\n" + a.styles.Help.Render(helpText))

	return b.String()
}

func (a *App) renderScheduleForm() string {
	var b strings.Builder

	// Header
	if a.formSchedule.ID == "" {
		b.WriteString(a.styles.Title.Render("新建播放计划") + "\n\n")
	} else {
		b.WriteString(a.styles.Title.Render("编辑播放计划") + "\n\n")
	}

	// Form fields
	fields := []struct {
		label string
		value string
	}{
		{"时间 (HH:MM)", a.timeInput},
		{"电台", func() string {
			if a.selectedStation != nil {
				return a.selectedStation.Title
			}
			return "点击选择电台"
		}()},
		{"类型", func() string {
			if a.recurring {
				return "每日重复"
			}
			return "单次播放"
		}()},
		{"状态", func() string {
			if a.formSchedule.Enabled {
				return "启用"
			}
			return "禁用"
		}()},
	}

	for i, field := range fields {
		labelStyle := lipgloss.NewStyle().Width(15)
		valueStyle := lipgloss.NewStyle()

		if a.stationSelecting && i == 1 {
			// Special handling for station selection
			b.WriteString(labelStyle.Render(field.label+":") + " " + a.styles.Selected.Render(field.value) + "\n")
		} else if !a.stationSelecting && i == 0 {
			// Time input field
			b.WriteString(labelStyle.Render(field.label+":") + " " + a.styles.Selected.Render(field.value) + "\n")
		} else {
			b.WriteString(labelStyle.Render(field.label+":") + " " + valueStyle.Render(field.value) + "\n")
		}
	}

	b.WriteString("\n")

	// Help text
	helpText := "↑/↓ 切换字段 | Enter 选择/确认 | Esc 取消 | Ctrl+S 保存"
	b.WriteString(a.styles.Help.Render(helpText))

	return b.String()
}

func (a *App) renderMainUI() string {
	var b strings.Builder

	leftWidth := 26
	rightWidth := 58
	gapWidth := 2
	if a.width > 0 {
		innerWidth := max(48, a.width-2)
		leftWidth = max(24, min(32, innerWidth/3))
		rightWidth = max(36, innerWidth-leftWidth-gapWidth)
	}
	contentHeight := max(8, a.height-3)

	provincePanel := a.renderProvincePanel(leftWidth, contentHeight)
	stationPanel := a.renderStationPanel(rightWidth, contentHeight)
	centerGap := lipgloss.NewStyle().Width(gapWidth).Height(contentHeight).Render("")

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, provincePanel, centerGap, stationPanel)
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

	statusWidth := leftWidth + gapWidth + rightWidth
	helpContent := fmt.Sprintf(
		"状态：%s | 省份：%s | 电台：%d | W/S 选台 | A/D 切省 | Enter/Space 播放 | X 停止 | R 刷新 | S 计划 | Q 退出",
		statusText,
		a.currentProvinceName(),
		len(a.stations),
	)
	help := a.styles.Help.Width(statusWidth).MaxWidth(statusWidth).Render(truncateRunes(helpContent, max(10, statusWidth)))
	b.WriteString("\n" + help)

	return b.String()
}

func (a *App) renderProvincePanel(width, height int) string {
	// 所有行统一用 rowStyle，宽度固定为 width，无额外 padding
	rowStyle := lipgloss.NewStyle().Width(width)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")).Width(width)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Width(width)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Width(width)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(width)

	var lines []string
	lines = append(lines, titleStyle.Render("省份"))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", max(1, width))))

	visibleHeight := max(1, height-5)
	start, end := windowRange(len(a.provinces), visibleHeight, a.provinceCursor)

	for i := start; i < end; i++ {
		province := a.provinces[i]
		if i == a.provinceCursor {
			lines = append(lines, selectedStyle.Render("> "+province.ProvinceName))
		} else {
			lines = append(lines, normalStyle.Render("  "+province.ProvinceName))
		}
	}

	for len(lines) < visibleHeight+2 {
		lines = append(lines, rowStyle.Render(" "))
	}

	return strings.Join(lines, "\n")
}

func (a *App) renderStationPanel(width, height int) string {
	// 所有行统一用 rowStyle，宽度固定为 width，无额外 padding
	rowStyle := lipgloss.NewStyle().Width(width)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")).Width(width)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Width(width)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Width(width)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Width(width)
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(width)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Width(width)

	visibleHeight := max(1, height-5)

	var lines []string
	lines = append(lines, titleStyle.Render("电台列表"))
	lines = append(lines, sepStyle.Render(strings.Repeat("─", max(1, width))))

	if len(a.stations) == 0 {
		emptyText := "当前省份暂无电台"
		if a.inlineLoading {
			emptyText = "正在切换省份..."
		}
		lines = append(lines, mutedStyle.Render(emptyText))

		// Ensure consistent height with province panel
		for len(lines) < visibleHeight+2 {
			lines = append(lines, rowStyle.Render(""))
		}

		return strings.Join(lines, "\n")
	}
	start, end := windowRange(len(a.stations), visibleHeight, a.cursor)

	for i := start; i < end; i++ {
		station := a.stations[i]
		stationTitle := "  " + station.Title
		if i == a.cursor {
			lines = append(lines, selectedStyle.Render(stationTitle))
			if station.Subtitle != "" {
				lines = append(lines, subtitleStyle.Render("  "+station.Subtitle))
			}
		} else {
			lines = append(lines, normalStyle.Render(stationTitle))
		}
	}

	for len(lines) < visibleHeight+2 {
		lines = append(lines, rowStyle.Render(" "))
	}

	if a.inlineLoading {
		lines = append(lines, mutedStyle.Render("正在切换省份..."))
	}
	return strings.Join(lines, "\n")
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

	start := max(0, cursor - visible/2)
	end := min(total, start + visible)

	// Adjust start if end was clamped
	if end - start < visible {
		start = max(0, end - visible)
	}

	return start, end
}

func (a *App) saveConfig() error {
	return config.SaveConfig(a.cfg)
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
