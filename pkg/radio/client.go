package radio

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	BaseURL          = "https://ytmsout.radio.cn"
	StationListPath  = "/web/appBroadcast/list"
	ProvinceListPath = "/web/appProvince/list/all"

	apiKey = "f0fc4c668392f9f9a447e48584c214ee"
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
	params := map[string]string{"categoryId": categoryID, "provinceCode": provinceCode}
	query := url.Values{"categoryId": {categoryID}, "provinceCode": {provinceCode}}
	requestURL := fmt.Sprintf("%s%s?%s", c.baseURL, StationListPath, query.Encode())

	resp, err := c.doRequest(requestURL, params)
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
	requestURL := fmt.Sprintf("%s%s", c.baseURL, ProvinceListPath)

	resp, err := c.doRequest(requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provinces: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code    int        `json:"code"`
		Message string     `json:"message"`
		Data    []Province `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("API returned error: %s", result.Message)
	}

	return result.Data, nil
}

func (c *Client) doRequest(url string, params map[string]string) (*http.Response, error) {
	timestamp := time.Now().UnixMilli()
	parts := make([]string, 0, len(params))
	for key, value := range params {
		parts = append(parts, key+"="+value)
	}
	sort.Strings(parts)
	canonical := strings.Join(parts, "&")
	if canonical != "" {
		canonical += "&"
	}
	canonical += fmt.Sprintf("timestamp=%d&key=%s", timestamp, apiKey)
	sum := md5.Sum([]byte(canonical))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("equipmentId", "0000")
	req.Header.Set("platformCode", "WEB")
	req.Header.Set("timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("sign", strings.ToUpper(hex.EncodeToString(sum[:])))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, fmt.Errorf("API returned HTTP status %s", resp.Status)
	}
	return resp, nil
}

// FindStationByID searches for a station by contentId.
// If provinceCode != 0, searches within that province.
// Otherwise, searches across all provinces.
func (c *Client) FindStationByID(stationID string, provinceCode int) (*Station, error) {
	if provinceCode != 0 {
		stations, err := c.GetStationsByFilter("0", fmt.Sprintf("%d", provinceCode))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch stations for province %d: %w", provinceCode, err)
		}
		for i := range stations {
			if stations[i].ContentID == stationID {
				return &stations[i], nil
			}
		}
		return nil, nil
	}

	// Deep search: try all provinces
	provinces, err := c.GetProvinces()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch provinces: %w", err)
	}

	// Search national stations first (provinceCode=0)
	stations, err := c.GetStationsByFilter("0", "0")
	if err == nil {
		for i := range stations {
			if stations[i].ContentID == stationID {
				return &stations[i], nil
			}
		}
	}

	// Search each province
	for _, prov := range provinces {
		stations, err := c.GetStationsByFilter("0", fmt.Sprintf("%d", prov.Code))
		if err != nil {
			continue
		}
		for i := range stations {
			if stations[i].ContentID == stationID {
				return &stations[i], nil
			}
		}
	}

	return nil, nil
}
