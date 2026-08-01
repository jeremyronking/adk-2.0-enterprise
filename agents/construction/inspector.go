package construction

import (
	"fmt"
	"math/rand"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

const InspectorName = "inspector_agent"
const InspectorDescription = "Inspects completed construction phases. Returns PASS if work passes inspection, or FIX if repairs are required."

type InspectArgs struct {
	Phase string `json:"phase" jsonschema:"The construction phase being inspected (e.g. 'foundation', 'framing', 'roofing', 'exterior', 'interior')."`
}

type InspectResult struct {
	Status   string `json:"status"`
	Phase    string `json:"phase"`
	Findings string `json:"findings"`
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func inspectPhase(ctx agent.Context, args InspectArgs) (InspectResult, error) {
	phase := args.Phase
	if phase == "" {
		phase = "construction"
	}

	// 75% PASS, 25% FIX
	if rng.Float32() < 0.75 {
		return InspectResult{
			Status:   "PASS",
			Phase:    phase,
			Findings: fmt.Sprintf("📋 [inspector_agent]: Phase %q APPROVED. All work meets building code standards.", phase),
		}, nil
	}

	return InspectResult{
		Status:   "FIX",
		Phase:    phase,
		Findings: fmt.Sprintf("📋 [inspector_agent]: Phase %q FAILED inspection. Minor defect detected. Repairs required before proceeding.", phase),
	}, nil
}

func NewInspectorTool() (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "inspect_phase",
		Description: "Evaluates a construction phase and returns PASS if approved or FIX if repairs are needed.",
	}, inspectPhase)
}

func NewInspectorAgent(m model.LLM) (agent.Agent, error) {
	t, err := NewInspectorTool()
	if err != nil {
		return nil, err
	}

	return llmagent.New(llmagent.Config{
		Name:        InspectorName,
		Model:       m,
		Description: InspectorDescription,
		Instruction: "You are the building inspector. Call inspect_phase with the active phase name. " +
			"Report the exact inspection finding in your output message.",
		Tools: []tool.Tool{t},
	})
}
