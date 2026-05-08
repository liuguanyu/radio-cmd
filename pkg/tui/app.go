package tui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/liuguanyu/radio-cmd/internal/config"
	"github.com/liuguanyu/radio-cmd/pkg/radio"
)

func debugLog(msg string) {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		home = "."
	}
	dir := filepath.Join(home, ".radio-cmd")
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "app.debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + msg + "\n")
}

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
	initialLoading bool

	// Schedule related fields
	currentView       ViewMode
	schedules         []config.Schedule
	scheduleCursor    int
	formSchedule      config.Schedule
	timeInput         string
	selectedStationID string
	scheduler         *Scheduler

	// Schedule form: province/station selection state
	formProvinceCursor  int
	formStationCursor   int
	formFieldCursor     int // 0=time, 1=province, 2=station, 3=enabled
	formProvinces       []radio.Province
	formStations        []radio.Station
	formStationsLoading bool

	// Channel for scheduler to notify about station switches
	stationSwitchedCh        chan StationSwitchedInfo
	schedulerTargetStationID string // used by scheduler to select station after province switch

	// Random station selection
	randomStationPending bool
}

// NewApp creates a new TUI application
func NewApp() *App {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	client := radio.NewClient()
	player := radio.NewPlayer()
	stationSwitchedCh := make(chan StationSwitchedInfo, 1)
	scheduler := NewScheduler(client, player, cfg, stationSwitchedCh)
	scheduler.Start()

	return &App{
		client:            client,
		player:            player,
		cfg:               cfg,
		styles:            NewStyles(),
		provinceFilter:    cfg.DefaultProvince,
		provinceCursor:    0,
		loading:           true,
		initialLoading:    true,
		currentView:       MainView,
		schedules:         cfg.Schedules,
		timeInput:         time.Now().Format("15:04"),
		scheduler:         scheduler,
		stationSwitchedCh: stationSwitchedCh,
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
	return a.fetchProvinces
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Non-blocking receive from scheduler station switch notifications
	select {
	case info := <-a.stationSwitchedCh:
		return a.handleStationSwitched(info)
	default:
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyPress(msg)

	case stationsFetchedMsg:
		// If the response doesn't match the currently selected province, discard it
		if msg.provinceFilter != a.provinceFilter {
			return a, nil
		}
		a.loading = false
		a.inlineLoading = false
		a.initialLoading = false
		if msg.err != nil {
			a.err = msg.err
			a.lastError = fmt.Sprintf("Failed to fetch stations: %v", msg.err)
			return a, nil
		}
		a.err = nil
		a.stations = msg.stations
		a.needsRefresh = false

		// If a target station was set (e.g., from scheduler), select it
		if a.schedulerTargetStationID != "" {
			for i, st := range a.stations {
				if st.ContentID == a.schedulerTargetStationID {
					a.cursor = i
					break
				}
			}
			// Clear after use
			a.schedulerTargetStationID = ""
		} else if a.randomStationPending {
			a.randomStationPending = false
			if len(a.stations) > 0 {
				a.cursor = rand.Intn(len(a.stations))
				station := &a.stations[a.cursor]
				if err := a.player.Play(station); err != nil {
					a.err = err
					a.lastError = err.Error()
				}
			}
		} else if len(a.stations) == 0 {
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

		if len(a.provinces) == 0 {
			return a, nil
		}

		// Sync provinceFilter to match the actual selected province,
		// then fetch stations for it.
		newFilter := strconv.Itoa(a.provinces[a.provinceCursor].Code)
		a.provinceFilter = newFilter
		a.cursor = 0
		a.stations = nil
		a.loading = true
		return a, a.fetchStationsWithFilter(newFilter)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case formStationsFetchedMsg:
		a.formStationsLoading = false
		if msg.err != nil {
			a.formStations = nil
		} else {
			a.formStations = msg.stations
			a.formStationCursor = 0
			// Only auto-select first station if selectedStationID is empty (NEW case)
			// For EDIT case, selectedStationID is pre-set to the schedule's station
			if len(a.formStations) > 0 && a.selectedStationID == "" {
				a.selectedStationID = a.formStations[0].ContentID
			}
			// Sync formStationCursor to match selectedStationID
			if a.selectedStationID != "" {
				for i, st := range a.formStations {
					if st.ContentID == a.selectedStationID {
						a.formStationCursor = i
						break
					}
				}
			}
		}
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
	stations       []radio.Station
	err            error
	provinceFilter string
}

type provincesFetchedMsg struct {
	provinces []radio.Province
	err       error
}

type formStationsFetchedMsg struct {
	stations []radio.Station
	err      error
}

// Commands
func (a *App) fetchStations() tea.Msg {
	return a.fetchStationsWithFilter(a.provinceFilter)
}

func (a *App) fetchStationsWithFilter(provinceFilter string) tea.Cmd {
	return func() tea.Msg {
		stations, err := a.client.GetStationsByFilter("0", provinceFilter)
		return stationsFetchedMsg{stations: stations, err: err, provinceFilter: provinceFilter}
	}
}

func (a *App) fetchProvinces() tea.Msg {
	provinces, err := a.client.GetProvinces()
	return provincesFetchedMsg{provinces: provinces, err: err}
}

func (a *App) fetchStationsForFormProvince(provinceCode int) tea.Cmd {
	return func() tea.Msg {
		stations, err := a.client.GetStationsByFilter("0", strconv.Itoa(provinceCode))
		return formStationsFetchedMsg{stations: stations, err: err}
	}
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

	case "f":
		return a.handleRandomStation()

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
			a.formSchedule = a.schedules[a.scheduleCursor]
			a.timeInput = a.formSchedule.Time
			a.formFieldCursor = 0
			a.formProvinceCursor = 0
			a.formStationCursor = 0
			a.formProvinces = append([]radio.Province{{Code: 0, ProvinceName: "国家台"}}, a.provinces...)
			a.formStations = nil
			a.formStationsLoading = false
			a.selectedStationID = a.formSchedule.StationID // Pre-set so findSelectedStation works after stations load
			a.lastError = ""
			a.currentView = ScheduleFormView
			// Load stations for the schedule's province
			var cmd tea.Cmd
			if a.formSchedule.ProvinceCode == 0 {
				cmd = a.fetchStationsForFormProvince(0)
			} else {
				cmd = a.fetchStationsForFormProvince(a.formSchedule.ProvinceCode)
				// Set province cursor to match
				for i, p := range a.formProvinces {
					if p.Code == a.formSchedule.ProvinceCode {
						a.formProvinceCursor = i
						break
					}
				}
			}
			return a, cmd
		}

	case "n", "N":
		// Create new schedule
		a.formSchedule = config.Schedule{
			ID:           uuid.New().String(),
			CreatedAt:    time.Now().Unix(),
			Enabled:      true,
			ProvinceCode: 0,
			Time:         time.Now().Format("15:04"),
		}
		a.timeInput = a.formSchedule.Time
		a.formFieldCursor = 0
		a.formProvinceCursor = 0
		a.formStationCursor = 0
		a.formProvinces = append([]radio.Province{{Code: 0, ProvinceName: "国家台"}}, a.provinces...)
		a.formStations = nil
		a.formStationsLoading = false
		a.selectedStationID = ""
		a.lastError = ""
		a.currentView = ScheduleFormView
		// Preload stations for first province (国家台 = 0)
		return a, a.fetchStationsForFormProvince(0)

	case "del", "delete":
		if len(a.schedules) > 0 {
			// Remove schedule
			a.schedules = append(a.schedules[:a.scheduleCursor], a.schedules[a.scheduleCursor+1:]...)
			if a.scheduleCursor >= len(a.schedules) && len(a.schedules) > 0 {
				a.scheduleCursor = len(a.schedules) - 1
			}
			a.cfg.Schedules = a.schedules
			a.saveConfig()
			a.scheduler.UpdateConfig(a.cfg)
		}
	}

	return a, nil
}

func (a *App) handleScheduleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.player.Stop()
		return a, tea.Quit

	case "ctrl+s", "ctrl+g":
		if err := a.saveScheduleForm(); err != nil {
			// keep lastError set, form will display it
		}
		return a, nil

	case "esc":
		a.currentView = ScheduleListView
		return a, nil

	case "tab":
		a.formFieldCursor = (a.formFieldCursor + 1) % 4
		return a, nil

	case "up":
		switch a.formFieldCursor {
		case 0: // Time
			a.adjustTime(-1)
		case 1: // Province
			if a.formProvinceCursor > 0 {
				a.formProvinceCursor--
			}
		case 2: // Station
			if a.formStationCursor > 0 {
				a.formStationCursor--
				if len(a.formStations) > 0 && a.formStationCursor < len(a.formStations) {
					a.selectedStationID = a.formStations[a.formStationCursor].ContentID
				}
			}
		case 3: // Enabled
			a.formSchedule.Enabled = !a.formSchedule.Enabled
		}

	case "down":
		switch a.formFieldCursor {
		case 0: // Time
			a.adjustTime(1)
		case 1: // Province
			if a.formProvinceCursor < len(a.formProvinces)-1 {
				a.formProvinceCursor++
			}
		case 2: // Station
			if a.formStationCursor < len(a.formStations)-1 {
				a.formStationCursor++
				if len(a.formStations) > 0 && a.formStationCursor < len(a.formStations) {
					a.selectedStationID = a.formStations[a.formStationCursor].ContentID
				}
			}
		case 3: // Enabled
			a.formSchedule.Enabled = !a.formSchedule.Enabled
		}

	case "left":
		switch a.formFieldCursor {
		case 1: // Province
			if a.formProvinceCursor > 0 {
				a.formProvinceCursor--
				a.formStationsLoading = true
				a.selectedStationID = ""
				return a, a.fetchStationsForFormProvince(a.formProvinceCode())
			}
		case 2: // Station
			if a.formStationCursor > 0 {
				a.formStationCursor--
				if len(a.formStations) > a.formStationCursor {
					a.selectedStationID = a.formStations[a.formStationCursor].ContentID
				}
			}
		case 3: // Enabled
			a.formSchedule.Enabled = !a.formSchedule.Enabled
		}

	case "right":
		switch a.formFieldCursor {
		case 1: // Province
			if a.formProvinceCursor < len(a.formProvinces)-1 {
				a.formProvinceCursor++
				a.formStationsLoading = true
				a.selectedStationID = ""
				return a, a.fetchStationsForFormProvince(a.formProvinceCode())
			}
		case 2: // Station
			if a.formStationCursor < len(a.formStations)-1 {
				a.formStationCursor++
				if len(a.formStations) > a.formStationCursor {
					a.selectedStationID = a.formStations[a.formStationCursor].ContentID
				}
			}
		case 3: // Enabled
			a.formSchedule.Enabled = !a.formSchedule.Enabled
		}

	case "enter":
		switch a.formFieldCursor {
		case 0, 1, 3:
			a.saveScheduleForm()
			return a, nil
		case 2: // Station: confirm selection and collapse list
			if len(a.formStations) > 0 && a.formStationCursor < len(a.formStations) {
				a.selectedStationID = a.formStations[a.formStationCursor].ContentID
				a.formFieldCursor = (a.formFieldCursor + 1) % 4
				return a, nil
			}
		}

	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", ":":
		if a.formFieldCursor == 0 {
			if len(a.timeInput) < 5 {
				a.timeInput += msg.String()
			}
		}

	case "backspace":
		if a.formFieldCursor == 0 && len(a.timeInput) > 0 {
			a.timeInput = a.timeInput[:len(a.timeInput)-1]
		}
	}

	return a, nil
}

func (a *App) applyProvinceSelection() tea.Cmd {
	if len(a.provinces) == 0 || a.provinceCursor >= len(a.provinces) {
		return nil
	}

	selected := a.provinces[a.provinceCursor]
	provinceFilter := strconv.Itoa(selected.Code)
	a.provinceFilter = provinceFilter
	a.cursor = 0
	a.stations = nil
	a.loading = true
	a.inlineLoading = true
	a.err = nil
	a.lastError = ""
	return a.fetchStationsWithFilter(provinceFilter)
}

func (a *App) handleRandomStation() (tea.Model, tea.Cmd) {
	if len(a.provinces) == 0 {
		return a, nil
	}
	randomIndex := rand.Intn(len(a.provinces))
	a.provinceCursor = randomIndex
	a.randomStationPending = true
	return a, a.applyProvinceSelection()
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
		b.WriteString(headerStyle.Render(fmt.Sprintf("%-10s %-20s %-10s %-10s", "时间", "电台", "省份", "状态")) + "\n")
		b.WriteString(strings.Repeat("─", 70) + "\n")

		for i, schedule := range a.schedules {
			var line string
			line = fmt.Sprintf("%-10s %-20s %-10s ", schedule.Time, schedule.StationName, provinceName(schedule.ProvinceCode, a.provinces))

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

	labelStyle := lipgloss.NewStyle().Width(12).Bold(true)
	activeValueStyle := a.styles.Selected
	normalValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Field 0: Time
	timeValue := a.timeInput
	if a.formFieldCursor == 0 {
		b.WriteString(labelStyle.Render("时间:") + " " + activeValueStyle.Render(timeValue) + "\n")
	} else {
		b.WriteString(labelStyle.Render("时间:") + " " + normalValueStyle.Render(timeValue) + "\n")
	}

	// Field 1: Province
	provName := formProvinceName(a.formProvinceCursor, a.formProvinces)
	if a.formFieldCursor == 1 {
		b.WriteString(labelStyle.Render("省份:") + " " + activeValueStyle.Render(provName) + "\n")
	} else {
		b.WriteString(labelStyle.Render("省份:") + " " + normalValueStyle.Render(provName) + "\n")
	}

	// Field 2: Station
	stationName := "请选择电台"
	if s := a.findSelectedStation(); s != nil {
		stationName = s.Title
	}
	if a.formStationsLoading {
		stationName = "加载中..."
	}
	if a.formFieldCursor == 2 {
		b.WriteString(labelStyle.Render("电台:") + " " + activeValueStyle.Render(stationName) + "\n")
	} else {
		b.WriteString(labelStyle.Render("电台:") + " " + normalValueStyle.Render(stationName) + "\n")
	}
	// Show station list when station field is active
	if a.formFieldCursor == 2 && len(a.formStations) > 0 {
		for i, st := range a.formStations {
			prefix := "  "
			if i == a.formStationCursor {
				prefix = "> "
				b.WriteString("    " + a.styles.Selected.Render(prefix+st.Title) + "\n")
			} else {
				b.WriteString("    " + normalValueStyle.Render(prefix+st.Title) + "\n")
			}
		}
	}

	// Field 3: Enabled
	enabledText := "启用"
	if !a.formSchedule.Enabled {
		enabledText = "禁用"
	}
	if a.formFieldCursor == 3 {
		b.WriteString(labelStyle.Render("状态:") + " " + activeValueStyle.Render(enabledText) + "\n")
	} else {
		b.WriteString(labelStyle.Render("状态:") + " " + normalValueStyle.Render(enabledText) + "\n")
	}

	b.WriteString("\n")
	if a.lastError != "" {
		if a.lastError == "保存成功！" {
			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
			b.WriteString(successStyle.Render("✓ "+a.lastError) + "\n\n")
		} else {
			b.WriteString(a.styles.Error.Render("✗ "+a.lastError) + "\n\n")
		}
	}
	helpText := "Tab 切换字段 | ↑↓ 修改 | ←→ 修改 | Enter 保存 | Esc 取消"
	b.WriteString(a.styles.Help.Render(helpText))

	return b.String()
}

func (a *App) renderMainUI() string {
	var b strings.Builder

	gapWidth := 2
	totalWidth := 86
	leftWidth := 24
	if a.width > 0 {
		totalWidth = max(48, a.width-2)
		leftWidth = max(20, min(24, totalWidth/4))
	}
	rightWidth := max(24, totalWidth-leftWidth-gapWidth)
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
		"状态：%s | 省份：%s | 电台：%d | W/S 选台 | A/D 切省 | Enter/Space 播放 | X 停止 | F 随机挑台 | R 刷新 | S 计划 | Q 退出",
		statusText,
		a.currentProvinceName(),
		len(a.stations),
	)
	help := a.styles.Help.Width(statusWidth).MaxWidth(statusWidth).Render(truncateRunes(helpContent, max(10, statusWidth)))
	b.WriteString("\n" + help)

	return b.String()
}

func (a *App) renderProvincePanel(width, height int) string {
	rowStyle := lipgloss.NewStyle().Width(width)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")).Width(width)
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Width(width)
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Width(width)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(width)

	var lines []string
	lines = append(lines, titleStyle.Render("省份"))
	lines = append(lines, sepStyle.Render(strings.Repeat("-", max(1, width))))

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
	lines = append(lines, sepStyle.Render(strings.Repeat("-", max(1, width))))

	if len(a.stations) == 0 {
		emptyText := "当前省份暂无电台"
		if a.initialLoading {
			emptyText = "正在加载..."
		} else if a.inlineLoading {
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

	start := max(0, cursor-visible/2)
	end := min(total, start+visible)

	// Adjust start if end was clamped
	if end-start < visible {
		start = max(0, end-visible)
	}

	return start, end
}

// formProvinceCode returns the province code for the currently selected form province.
func (a *App) formProvinceCode() int {
	if len(a.formProvinces) == 0 || a.formProvinceCursor >= len(a.formProvinces) {
		return 0
	}
	return a.formProvinces[a.formProvinceCursor].Code
}

// handleStationSwitched updates UI selection when scheduler triggers a station switch.
// It finds the correct province cursor, selects the province, and positions the cursor
// on the switched station.
func (a *App) handleStationSwitched(info StationSwitchedInfo) (tea.Model, tea.Cmd) {
	// Find province cursor matching ProvinceCode
	newProvinceCursor := -1
	for i, prov := range a.provinces {
		if prov.Code == info.ProvinceCode {
			newProvinceCursor = i
			break
		}
	}

	if newProvinceCursor == -1 {
		// Province not found, just update stations if already on correct province
		if fmt.Sprintf("%d", info.ProvinceCode) == a.provinceFilter {
			// Find station cursor
			for i, st := range a.stations {
				if st.ContentID == info.Station.ContentID {
					a.cursor = i
					break
				}
			}
		}
		return a, nil
	}

	// Check if already on the correct province
	provinceFilterStr := fmt.Sprintf("%d", info.ProvinceCode)
	if a.provinceFilter == provinceFilterStr {
		// Province already selected, just update station cursor
		for i, st := range a.stations {
			if st.ContentID == info.Station.ContentID {
				a.cursor = i
				break
			}
		}
		return a, nil
	}

	// Switch province and station
	a.provinceCursor = newProvinceCursor
	a.provinceFilter = provinceFilterStr
	a.cursor = 0
	a.stations = nil
	a.loading = true
	a.inlineLoading = true
	a.err = nil
	a.lastError = ""
	// Store the target station ID to select after stations load
	a.schedulerTargetStationID = info.Station.ContentID
	return a, a.fetchStationsWithFilter(provinceFilterStr)
}

// findSelectedStation returns the currently selected station by looking up
// selectedStationID in formStations. Returns nil if not found.
func (a *App) findSelectedStation() *radio.Station {
	if a.selectedStationID == "" {
		return nil
	}
	for i := range a.formStations {
		if a.formStations[i].ContentID == a.selectedStationID {
			return &a.formStations[i]
		}
	}
	return nil
}

// formProvinceName returns the province name for a given cursor position.
// formProvinces includes "国家台" (code=0) at index 0, followed by actual provinces.
func formProvinceName(cursor int, provinces []radio.Province) string {
	if cursor < 0 || cursor >= len(provinces) {
		return "国家台"
	}
	return provinces[cursor].ProvinceName
}

// provinceName returns the province name for a given province code.
func provinceName(code int, provinces []radio.Province) string {
	if code == 0 {
		return "国家台"
	}
	for _, p := range provinces {
		if p.Code == code {
			return p.ProvinceName
		}
	}
	return fmt.Sprintf("%d", code)
}

func (a *App) saveConfig() error {
	return config.SaveConfig(a.cfg)
}

func (a *App) saveScheduleForm() error {
	defer func() {
		if r := recover(); r != nil {
			debugLog(fmt.Sprintf("saveScheduleForm PANIC recovered: %v", r))
		}
	}()

	if a.formStationsLoading {
		a.lastError = "电台列表加载中，请稍候"
		debugLog("saveScheduleForm: stations still loading, cannot save")
		return fmt.Errorf("电台列表加载中")
	}

	s := a.findSelectedStation()
	if s == nil {
		a.lastError = "请先选择电台"
		debugLog(fmt.Sprintf("saveScheduleForm: station not found, selectedStationID=%q, formStations=%d", a.selectedStationID, len(a.formStations)))
		return fmt.Errorf("请先选择电台")
	}
	debugLog(fmt.Sprintf("saveScheduleForm: found station %s (%s)", s.Title, s.ContentID))
	a.formSchedule.StationID = s.ContentID
	a.formSchedule.StationName = s.Title
	a.formSchedule.Time = a.timeInput
	a.formSchedule.ProvinceCode = a.formProvinceCode()
	debugLog(fmt.Sprintf("saveScheduleForm: formSchedule StationID=%q StationName=%q Time=%q ProvinceCode=%d",
		a.formSchedule.StationID, a.formSchedule.StationName, a.formSchedule.Time, a.formSchedule.ProvinceCode))

	// Sync a.schedules and a.cfg.Schedules before modifying them
	if a.cfg.Schedules == nil {
		a.cfg.Schedules = a.schedules
	}
	debugLog(fmt.Sprintf("saveScheduleForm: before save - a.schedules len=%d cap=%d, cfg.Schedules len=%d cap=%d",
		len(a.schedules), cap(a.schedules), len(a.cfg.Schedules), cap(a.cfg.Schedules)))

	if a.formSchedule.ID == "" {
		a.formSchedule.ID = uuid.New().String()
		a.formSchedule.CreatedAt = time.Now().Unix()
		if cap(a.schedules) > 0 {
			a.schedules = append(a.schedules, a.formSchedule)
		} else {
			a.schedules = []config.Schedule{a.formSchedule}
		}
		debugLog(fmt.Sprintf("saveScheduleForm: NEW added, total schedules=%d", len(a.schedules)))
	} else {
		debugLog(fmt.Sprintf("saveScheduleForm: existing schedule update, searching for ID=%q", a.formSchedule.ID))
		found := false
		for i, sch := range a.schedules {
			if sch.ID == a.formSchedule.ID {
				a.schedules[i] = a.formSchedule
				found = true
				debugLog(fmt.Sprintf("saveScheduleForm: updated at index %d, total schedules=%d", i, len(a.schedules)))
				break
			}
		}
		if !found {
			// Schedule ID not found, add as new
			if cap(a.schedules) > 0 {
				a.schedules = append(a.schedules, a.formSchedule)
			} else {
				a.schedules = []config.Schedule{a.formSchedule}
			}
			debugLog(fmt.Sprintf("saveScheduleForm: ID not found, added as new, total=%d", len(a.schedules)))
		}
	}

	a.cfg.Schedules = a.schedules
	debugLog(fmt.Sprintf("saveScheduleForm: after save - a.schedules len=%d, cfg.Schedules len=%d",
		len(a.schedules), len(a.cfg.Schedules)))
	if err := a.saveConfig(); err != nil {
		a.lastError = fmt.Sprintf("保存失败: %v", err)
		debugLog(fmt.Sprintf("saveScheduleForm: saveConfig ERROR: %v", err))
		return err
	}
	debugLog(fmt.Sprintf("saveScheduleForm: SUCCESS, view switching to ScheduleListView"))
	a.scheduler.UpdateConfig(a.cfg)
	a.lastError = "保存成功！"
	a.currentView = ScheduleListView
	return nil
}

// adjustTime adjusts the time by delta minutes. delta can be -1 or +1.
func (a *App) adjustTime(delta int) {
	parts := strings.Split(a.timeInput, ":")
	if len(parts) != 2 {
		a.timeInput = "00:00"
		return
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		h = 0
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		m = 0
	}
	total := h*60 + m + delta
	if total < 0 {
		total = 24*60 + total
	}
	total = total % (24 * 60)
	a.timeInput = fmt.Sprintf("%02d:%02d", total/60, total%60)
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
