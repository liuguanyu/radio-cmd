package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liuguanyu/radio-cmd/internal/config"
	"github.com/liuguanyu/radio-cmd/pkg/radio"
)

// configDir returns the directory where config files are stored.
// On Windows it uses USERPROFILE, on Unix it uses HOME.
func configDir() string {
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
	return filepath.Join(home, ".radio-cmd")
}

// Scheduler checks schedules every minute and triggers station playback.
type Scheduler struct {
	client              *radio.Client
	player              *radio.Player
	cfg                 *config.Config
	mu                  sync.Mutex
	triggered           map[string]string // scheduleID -> triggerKey (year-dayofyear-hour-minute)
	stopCh              chan struct{}
	logFile             *os.File
	stationSwitchedCh   chan<- StationSwitchedInfo
}

// StationSwitchedInfo carries the station that was switched to.
type StationSwitchedInfo struct {
	Station      *radio.Station
	ProvinceCode int
}

// NewScheduler creates a new Scheduler instance.
func NewScheduler(client *radio.Client, player *radio.Player, cfg *config.Config, stationSwitchedCh chan<- StationSwitchedInfo) *Scheduler {
	return &Scheduler{
		client:            client,
		player:            player,
		cfg:               cfg,
		triggered:         make(map[string]string),
		stopCh:            make(chan struct{}),
		stationSwitchedCh: stationSwitchedCh,
	}
}

// Start launches the scheduler in a background goroutine.
func (s *Scheduler) Start() {
	go s.run()
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	if s.logFile != nil {
		s.logFile.Close()
	}
}

// UpdateConfig updates the config reference (call after config save).
func (s *Scheduler) UpdateConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

func (s *Scheduler) run() {
	// Align to the start of the next minute
	now := time.Now()
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	waitDuration := nextMinute.Sub(now)

	s.initLog()
	s.log("Scheduler starting, first check at %s", nextMinute.Format("15:04:05"))

	select {
	case <-time.After(waitDuration):
	case <-s.stopCh:
		return
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Check immediately at the aligned minute boundary
	s.check()

	for {
		select {
		case <-ticker.C:
			s.check()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) check() {
	s.mu.Lock()
	schedules := s.cfg.Schedules
	s.mu.Unlock()

	now := time.Now()
	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeMinutes := currentHour*60 + currentMinute

	var dueSchedules []config.Schedule

	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}

		parts := strings.Split(schedule.Time, ":")
		if len(parts) != 2 {
			continue
		}

		scheduleHour, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		scheduleMinute, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		scheduleTimeMinutes := scheduleHour*60 + scheduleMinute

		// 2-minute grace period (matching Kotlin ScheduleWorker)
		var timeDiff int
		if currentTimeMinutes >= scheduleTimeMinutes {
			timeDiff = currentTimeMinutes - scheduleTimeMinutes
		} else {
			timeDiff = (currentTimeMinutes + 24*60) - scheduleTimeMinutes
		}

		if timeDiff > 2 {
			continue
		}

		// Build trigger key: year-dayofyear-hour-minute
		targetTime := now
		if timeDiff > 0 && currentTimeMinutes < scheduleTimeMinutes {
			targetTime = now.AddDate(0, 0, -1)
		}
		triggerKey := fmt.Sprintf("%d-%d-%d-%d",
			targetTime.Year(), targetTime.YearDay(), scheduleHour, scheduleMinute)

		// Check if already triggered
		if prev, ok := s.triggered[schedule.ID]; ok && prev == triggerKey {
			continue
		}

		dueSchedules = append(dueSchedules, schedule)
		s.triggered[schedule.ID] = triggerKey
	}

	// Clean up old trigger keys (keep only last 100)
	if len(s.triggered) > 100 {
		s.triggered = make(map[string]string)
	}

	for _, schedule := range dueSchedules {
		s.log("Schedule triggered: %s -> %s at %s (current: %02d:%02d)",
			schedule.ID, schedule.StationName, schedule.Time, currentHour, currentMinute)
		s.triggerStationSwitch(schedule)
	}
}

func (s *Scheduler) triggerStationSwitch(schedule config.Schedule) {
	station, err := s.client.FindStationByID(schedule.StationID, schedule.ProvinceCode)
	if err != nil {
		s.log("Error finding station %s: %v", schedule.StationID, err)
		return
	}
	if station == nil {
		s.log("Station not found: %s (%s)", schedule.StationName, schedule.StationID)
		return
	}

	if err := s.player.Play(station); err != nil {
		s.log("Error playing station %s: %v", station.Title, err)
	} else {
		s.log("Switched to station: %s", station.Title)
		// Notify App to update UI selection
		if s.stationSwitchedCh != nil {
			select {
			case s.stationSwitchedCh <- StationSwitchedInfo{Station: station, ProvinceCode: schedule.ProvinceCode}:
			default:
				// Channel full, skip notification
			}
		}
	}
}

func (s *Scheduler) initLog() {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "scheduler.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	s.logFile = f
}

func (s *Scheduler) log(format string, args ...any) {
	msg := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
	if s.logFile != nil {
		s.logFile.WriteString(msg)
	}
}
