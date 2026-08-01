// Package weatheragent provides an agent that reports current conditions and
// a multi-day forecast for a place. It is designed to be wrapped as an
// agenttool by a parent agent.
package weatheragent

import (
	"fmt"
	"strconv"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"my-agent-enterprise/internal/openmeteo"
	"my-agent-enterprise/internal/registry"
)

// Name is the agent's name. When this agent is wrapped with agenttool.New,
// this becomes the function name the parent agent calls, so it has to be a
// valid identifier.
const Name = "weather_agent"

func init() {
	registry.Register(Name, Description, func(m model.LLM) (agent.Agent, error) {
		return New(m)
	})
}

// Description is what the parent agent sees when deciding whether to route a
// question here, so it is written for that audience rather than for a person.
//
// A var rather than a const because strconv.Itoa is not a constant
// expression; keeping the day count derived from openmeteo.ForecastDays is
// worth more than constness here.
var Description = "Reports current weather conditions and a " + strconv.Itoa(openmeteo.ForecastDays) +
	"-day forecast for a named city or place. Delegate any question about weather, " +
	"temperature, rain, or forecasts to this agent."

// LocationArgs is the input for get_weather.
type LocationArgs struct {
	Location string `json:"location" jsonschema:"The place to look up, as a city name optionally qualified by region or country. Examples: \"Paris\", \"Springfield, Illinois\", \"Auckland NZ\"."`
}

// CurrentConditions is a snapshot of the weather right now. Units are baked
// into the field names so the model never has to guess at them.
type CurrentConditions struct {
	Conditions      string  `json:"conditions"`
	TemperatureC    float64 `json:"temperature_c"`
	FeelsLikeC      float64 `json:"feels_like_c"`
	HumidityPercent int     `json:"humidity_percent"`
	PrecipitationMM float64 `json:"precipitation_mm"`
	WindSpeedKMH    float64 `json:"wind_speed_kmh"`
}

// DailyForecast is one day of the outlook.
type DailyForecast struct {
	Date                string  `json:"date"`
	Conditions          string  `json:"conditions"`
	TemperatureMinC     float64 `json:"temperature_min_c"`
	TemperatureMaxC     float64 `json:"temperature_max_c"`
	PrecipitationChance *int    `json:"precipitation_chance_percent,omitempty"`
}

// WeatherResult is what get_weather hands back to the model.
type WeatherResult struct {
	Status   string `json:"status"`
	Location string `json:"location,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	// ObservedAt is the local timestamp the current conditions apply to.
	// Open-Meteo quantises this to 15-minute intervals.
	ObservedAt   string             `json:"observed_at,omitempty"`
	Current      *CurrentConditions `json:"current,omitempty"`
	Forecast     []DailyForecast    `json:"forecast,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
}

// weather reports current conditions plus a multi-day forecast for a location.
//
// Lookup failures come back as a "error" status rather than a Go error, so
// the model can explain the problem and ask for a better location instead of
// the whole turn failing.
func weather(ctx agent.Context, args LocationArgs) (WeatherResult, error) {
	p, err := openmeteo.Geocode(ctx, args.Location)
	if err != nil {
		return WeatherResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	f, err := openmeteo.GetForecast(ctx, p)
	if err != nil {
		return WeatherResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	forecast := make([]DailyForecast, 0, len(f.Days))
	for _, d := range f.Days {
		forecast = append(forecast, DailyForecast{
			Date:                d.Date,
			Conditions:          openmeteo.DescribeCode(d.Code),
			TemperatureMinC:     d.TemperatureMinC,
			TemperatureMaxC:     d.TemperatureMaxC,
			PrecipitationChance: d.PrecipitationChance,
		})
	}

	return WeatherResult{
		Status:     "success",
		Location:   p.Label(),
		Timezone:   f.Timezone,
		ObservedAt: f.Current.Time,
		Current: &CurrentConditions{
			Conditions:      openmeteo.DescribeCode(f.Current.Code),
			TemperatureC:    f.Current.TemperatureC,
			FeelsLikeC:      f.Current.FeelsLikeC,
			HumidityPercent: f.Current.HumidityPercent,
			PrecipitationMM: f.Current.PrecipitationMM,
			WindSpeedKMH:    f.Current.WindSpeedKMH,
		},
		Forecast: forecast,
	}, nil
}

// NewTool builds the get_weather function tool.
func NewTool() (tool.Tool, error) {
	t, err := functiontool.New(functiontool.Config{
		Name: "get_weather",
		Description: "Returns the current weather conditions and a " + strconv.Itoa(openmeteo.ForecastDays) +
			"-day forecast for a location. Temperatures are in Celsius, wind speed in km/h, and precipitation in millimetres.",
	}, weather)
	if err != nil {
		return nil, fmt.Errorf("creating get_weather tool: %w", err)
	}
	return t, nil
}

// New builds the weather agent on the given model.
func New(m model.LLM) (agent.Agent, error) {
	weatherTool, err := NewTool()
	if err != nil {
		return nil, err
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        Name,
		Model:       m,
		Description: Description,
		Instruction: "You report weather for a place. Call get_weather with the location from " +
			"the request. Always name the location the tool resolved to, so an ambiguous city " +
			"can be corrected. If the tool returns status \"error\", explain the problem and ask " +
			"for a more specific location rather than guessing. Report temperatures in Celsius " +
			"unless asked otherwise. Answer only about weather; say so if asked for anything else.",
		Tools: []tool.Tool{weatherTool},
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", Name, err)
	}
	return a, nil
}
