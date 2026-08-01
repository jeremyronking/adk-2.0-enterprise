package construction

import (
	"fmt"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

const (
	FoundationName = "foundation_agent"
	FramingName    = "framing_agent"
	RoofingName    = "roofing_agent"
	ExteriorName   = "exterior_agent"
	InteriorName   = "interior_agent"
)

// TradeArgs represents the input for construction trade tools.
type TradeArgs struct {
	StructureType string `json:"structure_type" jsonschema:"The type of structure being built (e.g. 'home', 'mansion', 'shed', 'trailer', 'house')."`
	Action        string `json:"action,omitempty" jsonschema:"Optional action override: 'construct' (default) or 'repair'."`
}

// TradeResult represents the result of a trade tool invocation.
type TradeResult struct {
	Status  string `json:"status"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

func executeTrade(phase string, args TradeArgs) (TradeResult, error) {
	structName := strings.TrimSpace(args.StructureType)
	if structName == "" {
		structName = "structure"
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "repair" {
		return TradeResult{
			Status:  "success",
			Phase:   phase,
			Message: fmt.Sprintf("🛠️ [%s_agent]: Performing repairs on %s for %s... Repairs complete.", phase, phase, structName),
		}, nil
	}

	return TradeResult{
		Status:  "success",
		Phase:   phase,
		Message: fmt.Sprintf("🏗️ [%s_agent]: Starting %s construction on %s...\nFinished %s construction on %s.", phase, phase, structName, phase, structName),
	}, nil
}

// NewTradeTool creates a function tool for a specific construction phase.
func NewTradeTool(toolName, phase string) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        toolName,
		Description: fmt.Sprintf("Executes or repairs %s construction work for a structure.", phase),
	}, func(ctx agent.Context, args TradeArgs) (TradeResult, error) {
		return executeTrade(phase, args)
	})
}

// NewTradeAgent creates an LLM sub-agent for a specific construction phase.
func NewTradeAgent(name, phase, toolName string, m model.LLM) (agent.Agent, error) {
	t, err := NewTradeTool(toolName, phase)
	if err != nil {
		return nil, err
	}

	return llmagent.New(llmagent.Config{
		Name:        name,
		Model:       m,
		Description: fmt.Sprintf("Handles %s construction and repairs for a structure.", phase),
		Instruction: fmt.Sprintf("You are the %s. Call %s with the structure type. "+
			"State clearly in your output message that you have started and finished %s construction (or repairs if requested).", name, toolName, phase),
		Tools: []tool.Tool{t},
	})
}
