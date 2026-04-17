# Radio.cn CLI Player

A Go CLI+TUI application for playing online radio stations from radio.cn

## Features

- 📻 Browse 19+ Chinese radio stations from radio.cn
- 🎧 Play audio streams using system audio players (mpv, vlc, ffplay)
- 🖥️ Beautiful TUI interface with BubbleTea and Lipgloss
- 📊 Automatic station list fetching via radio.cn API
- ⚙️ Configurable settings with persistent configuration
- 🚀 Pure Go implementation with minimal dependencies

## Installation

### Prerequisites

Install a compatible audio player:

**macOS**:
```bash
brew install mpv
```

**Linux**:
```bash
sudo apt install mpv
```

**Windows**:
Download mpv from https://mpv.io/installation/

### Install the application

```bash
go install github.com/liuguanyu/radio-cmd@latest
```

## Usage

```bash
radio-cmd
```

### Keyboard Controls

- ↑/↓ or k/j: Navigate stations
- Enter/Space: Play/Pause station
- S: Stop playback
- R: Refresh station list
- Q: Quit application

## Configuration

Configuration is stored in `~/.radio-cmd/config.json`:

```json
{
  "default_category": "0",
  "default_province": "0",
  "use_high_quality": true,
  "auto_refresh": false,
  "refresh_interval_minutes": 30,
  "max_failed_retries": 3,
  "cache_stations": true,
  "cache_ttl_minutes": 60
}
```

## Logs

Player output (including any errors from the audio player like mpv/ffplay) is logged to `~/.radio-cmd/player.log` to keep the TUI interface clean. Check this file if you experience playback issues.

## Troubleshooting

### No sound?
- Check that your audio player is installed (mpv/ffplay/vlc)
- Look at `~/.radio-cmd/player.log` for errors
- Ensure your system audio is working
- Some URLs may require internet connectivity

### Player not found?
Install one of the supported players:
- **macOS**: `brew install mpv`
- **Linux**: `sudo apt install mpv` or `sudo apt install vlc`
- **Windows**: Download mpv from https://mpv.io/installation/

### Build errors?
Make sure you have Go 1.21+ installed:
```bash
go version
```

## Development

To develop locally:

```bash
go build -o radio-cmd ./cmd/radio-cmd
./radio-cmd
```

## License

MIT
