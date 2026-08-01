package construction

import (
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/agenttool"

	"my-agent-enterprise/internal/registry"
)

const Name = "construction_agent"
const Description = "Orchestrates the multi-agent construction workflow (foundation, framing, roofing, exterior, interior) with phase inspections for any structure building request."

func init() {
	registry.Register(Name, Description, func(m model.LLM) (agent.Agent, error) {
		return New(m)
	})
}

// New builds the construction orchestrator agent wrapping sub-agents.
func New(m model.LLM) (agent.Agent, error) {
	foundationSub, err := NewTradeAgent(FoundationName, "foundation", "build_foundation", m)
	if err != nil {
		return nil, fmt.Errorf("creating foundation agent: %w", err)
	}

	framingSub, err := NewTradeAgent(FramingName, "framing", "build_framing", m)
	if err != nil {
		return nil, fmt.Errorf("creating framing agent: %w", err)
	}

	roofingSub, err := NewTradeAgent(RoofingName, "roofing", "build_roofing", m)
	if err != nil {
		return nil, fmt.Errorf("creating roofing agent: %w", err)
	}

	exteriorSub, err := NewTradeAgent(ExteriorName, "exterior", "build_exterior", m)
	if err != nil {
		return nil, fmt.Errorf("creating exterior agent: %w", err)
	}

	interiorSub, err := NewTradeAgent(InteriorName, "interior", "build_interior", m)
	if err != nil {
		return nil, fmt.Errorf("creating interior agent: %w", err)
	}

	inspectorSub, err := NewInspectorAgent(m)
	if err != nil {
		return nil, fmt.Errorf("creating inspector agent: %w", err)
	}

	instruction := "You are the Construction Master Orchestrator. When a user requests to build or construct a structure (home, mansion, shed, trailer, house, etc.):\n\n" +
		"Execute the construction workflow step-by-step across 5 phases: foundation, framing, roofing, exterior, interior.\n" +
		"CRITICAL INSTRUCTION: You MUST output a text message describing the result after EVERY SINGLE SUB-AGENT CALL before proceeding to the next step.\n\n" +
		"For each phase (foundation ➔ framing ➔ roofing ➔ exterior ➔ interior):\n" +
		"  Step A: Call the corresponding trade sub-agent (" + FoundationName + ", " + FramingName + ", " + RoofingName + ", " + ExteriorName + ", " + InteriorName + ") to construct the phase.\n" +
		"  Step B: Call " + InspectorName + " with the phase name to inspect the work.\n" +
		"  Step C: If " + InspectorName + " returns status 'FIX', call the trade sub-agent again with action='repair', then call " + InspectorName + " again to re-inspect.\n" +
		"  Step D: Only move to the next phase after status 'PASS' is achieved.\n\n" +
		"After all 5 phases pass inspection, state that construction is complete."

	a, err := llmagent.New(llmagent.Config{
		Name:        Name,
		Model:       m,
		Description: Description,
		Instruction: instruction,
		Tools: []tool.Tool{
			agenttool.New(foundationSub, nil),
			agenttool.New(framingSub, nil),
			agenttool.New(roofingSub, nil),
			agenttool.New(exteriorSub, nil),
			agenttool.New(interiorSub, nil),
			agenttool.New(inspectorSub, nil),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", Name, err)
	}
	return a, nil
}
