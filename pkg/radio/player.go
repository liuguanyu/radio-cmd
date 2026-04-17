package radio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PlayerState represents the current state of the player
type PlayerState int

const (
	StateStopped PlayerState = iota
	StatePlaying
	StatePaused
)

// Player controls audio playback using system players
type Player struct {
	cmd    *exec.Cmd
	state  PlayerState
	Station *Station
}

// NewPlayer creates a new player instance
func NewPlayer() *Player {
	return &Player{
		state: StateStopped,
	}
}

// Play starts playing the given station using available system players
func (p *Player) Play(station *Station) error {
	if p.state == StatePlaying {
		p.Stop()
	}

	url := station.GetBestPlayURL()
	if url == "" {
		return fmt.Errorf("no playable URL available for station %s", station.Title)
	}

	// Try different players based on OS
	var cmd *exec.Cmd
	var err error

	switch runtime.GOOS {
	case "darwin":
		cmd, err = p.createMacCommand(url)
	case "linux":
		cmd, err = p.createLinuxCommand(url)
	case "windows":
		cmd, err = p.createWindowsCommand(url)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return fmt.Errorf("failed to create player command: %w", err)
	}

	// Create log file in config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".radio-cmd")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(configDir, "player.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Redirect player output to log file instead of TUI
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start player: %w", err)
	}

	p.cmd = cmd
	p.state = StatePlaying
	p.Station = station

	return nil
}

func (p *Player) createMacCommand(url string) (*exec.Cmd, error) {
	// For macOS, try mpv first, then ffplay from ffmpeg
	if p.hasCommand("mpv") {
		return exec.Command("mpv", "--no-video", "--quiet", url), nil
	}

	if p.hasCommand("ffplay") {
		return exec.Command("ffplay", "-nodisp", "-autoexit", "-i", url), nil
	}

	if p.hasCommand("vlc") {
		return exec.Command("vlc", "--intf", "dummy", "--play-and-exit", url), nil
	}

	return nil, fmt.Errorf("please install an audio player: mpv (brew install mpv), ffmpeg (brew install ffmpeg), or vlc")
}

func (p *Player) createLinuxCommand(url string) (*exec.Cmd, error) {
	// For Linux, try mpv, vlc, or ffplay
	if p.hasCommand("mpv") {
		return exec.Command("mpv", "--no-video", "--quiet", url), nil
	}

	if p.hasCommand("cvlc") {
		return exec.Command("cvlc", "--play-and-exit", "--intf", "dummy", url), nil
	}

	if p.hasCommand("ffplay") {
		return exec.Command("ffplay", "-nodisp", "-autoexit", "-i", url), nil
	}

	if p.hasCommand("aplay") {
		return exec.Command("aplay", url), nil
	}

	return nil, fmt.Errorf("please install an audio player: sudo apt install mpv, vlc, ffmpeg, or alsa-utils")
}

func (p *Player) createWindowsCommand(url string) (*exec.Cmd, error) {
	// For Windows, check if using Windows Subsystem for Linux (WSL)
	// or native Windows with different players

	// Try command-line players first
	if p.hasCommand("mpv.exe") {
		return exec.Command("mpv", "--no-video", "--quiet", url), nil
	}

	if p.hasCommand("ffplay.exe") {
		return exec.Command("ffplay", "-nodisp", "-autoexit", "-i", url), nil
	}

	// For PowerShell based playback using Windows Media Player
	return exec.Command("powershell", "-Command", `
$player = New-Object -ComObject WMPlayer.OCX
$media = $player.newMedia("` + url + `")
$player.currentPlaylist = $player.newPlaylist("Radio", "Radio")
$player.currentPlaylist.appendItem($media)
$player.controls.play()
while ($player.playstate -eq 3) { Start-Sleep -Milliseconds 200 }
`), nil
}

// Stop stops the current playback
func (p *Player) Stop() error {
	if p.cmd != nil && p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop player: %w", err)
		}
		p.cmd.Wait()
	}
	p.state = StateStopped
	p.Station = nil
	p.cmd = nil
	return nil
}

// GetState returns the current player state
func (p *Player) GetState() PlayerState {
	return p.state
}

// IsPlaying returns true if the player is currently playing
func (p *Player) IsPlaying() bool {
	return p.state == StatePlaying
}

// hasCommand checks if a command is available in the system PATH
func (p *Player) hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// String returns a string representation of the player state
func (p *Player) String() string {
	var stateStr string
	switch p.state {
	case StatePlaying:
		stateStr = "Playing"
	case StatePaused:
		stateStr = "Paused"
	default:
		stateStr = "Stopped"
	}

	if p.Station != nil {
		return fmt.Sprintf("%s: %s", stateStr, p.Station.Title)
	}
	return stateStr
}

// CheckDependencies verifies that required audio players are available
func CheckDependencies() error {
	var missing []string
	runtimeOS := runtime.GOOS

	if runtimeOS == "darwin" && !hasCommand("mpv") && !hasCommand("ffplay") && !hasCommand("vlc") {
		missing = append(missing, "On macOS: install with 'brew install mpv' or 'brew install ffmpeg'")
	} else if runtimeOS == "linux" && !hasCommand("mpv") && !hasCommand("cvlc") && !hasCommand("ffplay") && !hasCommand("aplay") {
		missing = append(missing, "On Linux: install with 'sudo apt install mpv' or 'sudo apt install vlc'")
	} else if runtimeOS == "windows" && !hasCommand("mpv.exe") && !hasCommand("ffplay.exe") {
		missing = append(missing, "On Windows: install mpv from https://mpv.io/installation/")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing audio player:\n%s", lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(strings.Join(missing, "\n")))
	}

	return nil
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
