package radio

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetStationsByFilterSignsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("equipmentId"); got != "0000" {
			t.Errorf("equipmentId = %q, want 0000", got)
		}
		if got := r.Header.Get("platformCode"); got != "WEB" {
			t.Errorf("platformCode = %q, want WEB", got)
		}

		timestamp := r.Header.Get("timestamp")
		canonical := "categoryId=0&provinceCode=110000&timestamp=" + timestamp + "&key=" + apiKey
		sum := md5.Sum([]byte(canonical))
		wantSign := strings.ToUpper(hex.EncodeToString(sum[:]))
		if got := r.Header.Get("sign"); got != wantSign {
			t.Errorf("sign = %q, want %q", got, wantSign)
		}

		fmt.Fprint(w, `{"code":0,"message":"SUCCESS","data":[{"contentId":"639","title":"中国之声"}]}`)
	}))
	defer server.Close()

	client := &Client{httpClient: &http.Client{Timeout: time.Second}, baseURL: server.URL}
	stations, err := client.GetStationsByFilter("0", "110000")
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 || stations[0].ContentID != "639" {
		t.Fatalf("unexpected stations: %#v", stations)
	}
}

func TestGetProvincesReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":1001,"message":"参数不合法"}`)
	}))
	defer server.Close()

	client := &Client{httpClient: &http.Client{Timeout: time.Second}, baseURL: server.URL}
	_, err := client.GetProvinces()
	if err == nil || !strings.Contains(err.Error(), "参数不合法") {
		t.Fatalf("expected API error, got %v", err)
	}
}
