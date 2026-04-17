package radio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL          = "https://ytmsout.radio.cn"
	StationListPath  = "/web/appBroadcast/list"
	ProvinceListPath = "/web/appProvince/list/all"
)

// Province represents a province for regional stations
type Province struct {
	Code         int    `json:"provinceCode"`
	ProvinceName string `json:"provinceName"`
}

// Client is the radio.cn API client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: BaseURL,
	}
}

// GetStations fetches the list of radio stations
// categoryId: 0 for all categories
// provinceCode: 0 for all provinces
func (c *Client) GetStations() ([]Station, error) {
	return c.GetStationsByFilter("0", "0")
}

// GetStationsByFilter fetches stations filtered by category and province
func (c *Client) GetStationsByFilter(categoryID, provinceCode string) ([]Station, error) {
	url := fmt.Sprintf("%s%s?categoryId=%s&provinceCode=%s", c.baseURL, StationListPath, categoryID, provinceCode)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stations: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API returned error: %s", apiResp.Message)
	}

	return apiResp.Data, nil
}

// GetProvinces fetches the list of provinces
func (c *Client) GetProvinces() ([]Province, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, ProvinceListPath)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provinces: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code int        `json:"code"`
		Data []Province `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Data, nil
}
