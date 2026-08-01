// Package openmeteo is a small client for the Open-Meteo geocoding and
// forecast APIs. It is free for non-commercial use and needs no API key.
//
// Docs: https://open-meteo.com/en/docs
//
// The package deliberately knows nothing about ADK: it takes a plain
// context.Context and returns plain structs, so the agent packages own the
// shape of whatever they hand back to the model.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	geocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"
	forecastURL = "https://api.open-meteo.com/v1/forecast"

	// ForecastDays is how many days of outlook Forecast requests. Exported so
	// the weather agent can quote the number in its tool description.
	ForecastDays = 7
)

// httpClient is shared across calls so connections are reused.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// Place is one result from the geocoding API.
type Place struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	Admin1    string  `json:"admin1"`
	Timezone  string  `json:"timezone"`
}

// Label renders a place the way a person would write it: "Bangkok, Thailand",
// or "Springfield, Illinois, United States" when the region disambiguates.
func (p Place) Label() string {
	parts := []string{p.Name}
	if p.Admin1 != "" && !strings.EqualFold(p.Admin1, p.Name) {
		parts = append(parts, p.Admin1)
	}
	if p.Country != "" {
		parts = append(parts, p.Country)
	}
	return strings.Join(parts, ", ")
}

// ErrPlaceNotFound is returned when geocoding succeeds but matches nothing.
type ErrPlaceNotFound struct{ Query string }

func (e ErrPlaceNotFound) Error() string {
	return fmt.Sprintf("no place matching %q was found", e.Query)
}

// Conditions is a snapshot of the weather right now. Open-Meteo quantises
// Time to 15-minute intervals, so it is not a precise clock reading.
type Conditions struct {
	Time            string
	Code            int
	TemperatureC    float64
	FeelsLikeC      float64
	HumidityPercent int
	PrecipitationMM float64
	WindSpeedKMH    float64
}

// Day is one day of the outlook. PrecipitationChance is nil when the API
// omits a probability for that day.
type Day struct {
	Date                string
	Code                int
	TemperatureMinC     float64
	TemperatureMaxC     float64
	PrecipitationChance *int
}

// Forecast is current conditions plus a multi-day outlook, with every
// timestamp already local to the place being asked about.
type Forecast struct {
	Timezone string
	Current  Conditions
	Days     []Day
}

// Geocode turns a free-text location into coordinates and an IANA timezone.
// Only the top match is used; callers should report Place.Label back to the
// user so a wrong guess can be spotted.
func Geocode(ctx context.Context, location string) (Place, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return Place{}, fmt.Errorf("no location was given")
	}

	q := url.Values{
		"name":     {location},
		"count":    {"1"},
		"language": {"en"},
		"format":   {"json"},
	}
	var body struct {
		Results []Place `json:"results"`
	}
	if err := getJSON(ctx, geocodeURL, q, &body); err != nil {
		return Place{}, err
	}
	if len(body.Results) == 0 {
		return Place{}, ErrPlaceNotFound{Query: location}
	}
	return body.Results[0], nil
}

// forecastPayload mirrors the subset of the Open-Meteo forecast response we
// request. Daily precipitation probability is nullable, hence the pointers.
type forecastPayload struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Time          string  `json:"time"`
		Temperature2M float64 `json:"temperature_2m"`
		Humidity2M    int     `json:"relative_humidity_2m"`
		ApparentTemp  float64 `json:"apparent_temperature"`
		Precipitation float64 `json:"precipitation"`
		WeatherCode   int     `json:"weather_code"`
		WindSpeed10M  float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Time          []string  `json:"time"`
		WeatherCode   []int     `json:"weather_code"`
		TempMax       []float64 `json:"temperature_2m_max"`
		TempMin       []float64 `json:"temperature_2m_min"`
		PrecipProbMax []*int    `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// GetForecast fetches current conditions plus a multi-day outlook for a place.
// It is not named Forecast because that name is taken by the return type.
func GetForecast(ctx context.Context, p Place) (Forecast, error) {
	q := url.Values{
		"latitude":  {strconv.FormatFloat(p.Latitude, 'f', 4, 64)},
		"longitude": {strconv.FormatFloat(p.Longitude, 'f', 4, 64)},
		"current":   {"temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m"},
		"daily":     {"weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max"},
		// "auto" makes the API resolve the timezone from the coordinates, so
		// every timestamp comes back local to the place being asked about.
		"timezone":      {"auto"},
		"forecast_days": {strconv.Itoa(ForecastDays)},
	}
	var payload forecastPayload
	if err := getJSON(ctx, forecastURL, q, &payload); err != nil {
		return Forecast{}, err
	}

	timezone := payload.Timezone
	if timezone == "" {
		timezone = p.Timezone
	}

	// The daily arrays are parallel; the shortest one bounds the loop so a
	// truncated response can't panic.
	n := len(payload.Daily.Time)
	n = min(n, len(payload.Daily.WeatherCode))
	n = min(n, len(payload.Daily.TempMax))
	n = min(n, len(payload.Daily.TempMin))

	days := make([]Day, 0, n)
	for i := range n {
		day := Day{
			Date:            payload.Daily.Time[i],
			Code:            payload.Daily.WeatherCode[i],
			TemperatureMinC: payload.Daily.TempMin[i],
			TemperatureMaxC: payload.Daily.TempMax[i],
		}
		if i < len(payload.Daily.PrecipProbMax) {
			day.PrecipitationChance = payload.Daily.PrecipProbMax[i]
		}
		days = append(days, day)
	}

	return Forecast{
		Timezone: timezone,
		Current: Conditions{
			Time:            payload.Current.Time,
			Code:            payload.Current.WeatherCode,
			TemperatureC:    payload.Current.Temperature2M,
			FeelsLikeC:      payload.Current.ApparentTemp,
			HumidityPercent: payload.Current.Humidity2M,
			PrecipitationMM: payload.Current.Precipitation,
			WindSpeedKMH:    payload.Current.WindSpeed10M,
		},
		Days: days,
	}, nil
}

// getJSON performs a GET and decodes the JSON body into out.
func getJSON(ctx context.Context, endpoint string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", endpoint, err)
	}
	return nil
}

// weatherCodes maps WMO weather interpretation codes to plain English.
// See the "WMO Weather interpretation codes" table in the Open-Meteo docs.
var weatherCodes = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	56: "Light freezing drizzle",
	57: "Dense freezing drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	66: "Light freezing rain",
	67: "Heavy freezing rain",
	71: "Slight snowfall",
	73: "Moderate snowfall",
	75: "Heavy snowfall",
	77: "Snow grains",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers",
	86: "Heavy snow showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

// DescribeCode turns a WMO weather interpretation code into plain English.
func DescribeCode(code int) string {
	if desc, ok := weatherCodes[code]; ok {
		return desc
	}
	return fmt.Sprintf("Unknown conditions (WMO code %d)", code)
}
