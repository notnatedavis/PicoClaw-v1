//   pkg/tools/weather.go

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/picoclaw/pkg/llm"
)

// WeatherTool fetches current weather from Open-Meteo (no API key).
type WeatherTool struct{}

func (t *WeatherTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	city, _ := params["city"].(string)
	lat, _ := params["latitude"].(float64)
	lon, _ := params["longitude"].(float64)

	if city == "" && (lat == 0 || lon == 0) {
		return nil, fmt.Errorf("provide either 'city' or both 'latitude' and 'longitude'")
	}

	if city != "" {
		// Geocode city to coordinates
		geocodeURL := "https://geocoding-api.open-meteo.com/v1/search?name=" + url.QueryEscape(city) + "&count=1"
		resp, err := http.Get(geocodeURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var geoResp struct {
			Results []struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
				Name      string  `json:"name"`
				Country   string  `json:"country"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&geoResp); err != nil {
			return nil, err
		}
		if len(geoResp.Results) == 0 {
			return nil, fmt.Errorf("city not found")
		}
		lat = geoResp.Results[0].Latitude
		lon = geoResp.Results[0].Longitude
	}

	weatherURL := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m", lat, lon)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(weatherURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var weatherResp struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
			Humidity    float64 `json:"relative_humidity_2m"`
			WeatherCode int     `json:"weather_code"`
			WindSpeed   float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"temperature_c": weatherResp.Current.Temperature,
		"humidity_%":    weatherResp.Current.Humidity,
		"weather_code":  weatherResp.Current.WeatherCode,
		"wind_speed_kmh": weatherResp.Current.WindSpeed,
	}, nil
}

func (t *WeatherTool) Describe() llm.ToolDef {
	return llm.ToolDef{
		Name:        "weather",
		Description: "Get current weather for a city or coordinates using Open-Meteo (free, no key).",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"city":      map[string]interface{}{"type": "string"},
				"latitude":  map[string]interface{}{"type": "number"},
				"longitude": map[string]interface{}{"type": "number"},
			},
		},
	}
}