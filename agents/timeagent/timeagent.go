// Package timeagent provides an agent that reports the current local time
// at a place. It is designed to be wrapped as an agenttool by a parent agent.
package timeagent

import (
	"fmt"
	"time"

	// Embed the IANA timezone database so local-time lookups work on hosts
	// that ship without /usr/share/zoneinfo (scratch containers, Windows).
	_ "time/tzdata"

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
const Name = "time_agent"

func init() {
	registry.Register(Name, Description, func(m model.LLM) (agent.Agent, error) {
		return New(m)
	})
}

// Description is what the parent agent sees when deciding whether to route a
// question here, so it is written for that audience rather than for a person.
const Description = "Reports the current local date, time, and timezone for a named city or place. " +
	"Delegate any question about what time it is somewhere to this agent."

// LocationArgs is the input for get_current_time.
type LocationArgs struct {
	Location string `json:"location" jsonschema:"The place to look up, as a city name optionally qualified by region or country. Examples: \"Paris\", \"Springfield, Illinois\", \"Auckland NZ\"."`
}

// TimeResult is what get_current_time hands back to the model.
type TimeResult struct {
	Status string `json:"status"`
	// Location is the resolved place name, which may differ from what was
	// asked for -- the model should quote it back so the user can correct us.
	Location     string `json:"location,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	CurrentTime  string `json:"current_time,omitempty"`
	UTCOffset    string `json:"utc_offset,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// currentTime reports the wall-clock time at a location. It only needs the
// geocoding call: that response carries the IANA timezone, which gives an
// exact time rather than the forecast endpoint's 15-minute-quantised stamp.
//
// Lookup failures come back as a "error" status rather than a Go error, so
// the model can explain the problem and ask for a better location instead of
// the whole turn failing.
func currentTime(ctx agent.Context, args LocationArgs) (TimeResult, error) {
	p, err := openmeteo.Geocode(ctx, args.Location)
	if err != nil {
		return TimeResult{Status: "error", ErrorMessage: err.Error()}, nil
	}

	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return TimeResult{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("unknown timezone %q for %s", p.Timezone, p.Label()),
		}, nil
	}

	now := time.Now().In(loc)
	return TimeResult{
		Status:      "success",
		Location:    p.Label(),
		Timezone:    p.Timezone,
		CurrentTime: now.Format("2006-01-02 15:04:05 MST"),
		UTCOffset:   now.Format("-07:00"),
	}, nil
}

// NewTool builds the get_current_time function tool.
func NewTool() (tool.Tool, error) {
	t, err := functiontool.New(functiontool.Config{
		Name:        "get_current_time",
		Description: "Returns the current local date and time at a location, along with its timezone. Use this for any question about what time it is somewhere.",
	}, currentTime)
	if err != nil {
		return nil, fmt.Errorf("creating get_current_time tool: %w", err)
	}
	return t, nil
}

// New builds the time agent on the given model.
func New(m model.LLM) (agent.Agent, error) {
	timeTool, err := NewTool()
	if err != nil {
		return nil, err
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        Name,
		Model:       m,
		Description: Description,
		Instruction: "You report the current time at a place. Call get_current_time with the " +
			"location from the request. Always name the location the tool resolved to, so an " +
			"ambiguous city can be corrected. If the tool returns status \"error\", explain the " +
			"problem and ask for a more specific location rather than guessing. Answer only " +
			"about time; say so if asked for anything else.",
		Tools: []tool.Tool{timeTool},
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", Name, err)
	}
	return a, nil
}
