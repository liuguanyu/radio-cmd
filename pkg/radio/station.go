package radio

// Station represents a radio station from radio.cn API
type Station struct {
	ContentID      string `json:"contentId"`
	Title          string `json:"title"`
	Subtitle       string `json:"subtitle"`
	Image          string `json:"image"`
	PlayUrlLow     string `json:"playUrlLow"`
	Mp3PlayUrlLow  string `json:"mp3PlayUrlLow"`
	Mp3PlayUrlHigh string `json:"mp3PlayUrlHigh"`
	PlayUrlMulti   string `json:"playUrlMulti"`
}

// APIResponse represents the API response structure
type APIResponse struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    []Station  `json:"data"`
}

// GetBestPlayURL returns the best available playback URL
func (s *Station) GetBestPlayURL() string {
	// Prefer high quality MP3, then low quality, then HLS
	if s.Mp3PlayUrlHigh != "" {
		return s.Mp3PlayUrlHigh
	}
	if s.Mp3PlayUrlLow != "" {
		return s.Mp3PlayUrlLow
	}
	if s.PlayUrlLow != "" {
		return s.PlayUrlLow
	}
	return s.PlayUrlMulti
}
