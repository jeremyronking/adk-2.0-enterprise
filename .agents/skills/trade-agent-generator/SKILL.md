---
name: trade-agent-generator
description: Scaffolds a new trade sub-agent or construction phase within the construction orchestrator in adk-2.0-enterprise.
---

# Construction Trade Sub-Agent Generator Skill

Use this skill when extending the `adk-enterprise-construction` microservice with a new construction trade or phase sub-agent.

---

## Workflow Steps

### Step 1: Define Trade Constants
In [`agents/construction/trades.go`](../../agents/construction/trades.go), define trade phase constants and names:

```go
const (
	Trade<PhaseName>Name = "trade_<phasename>"
	Phase<PhaseName>     = "<Phase Description>"
	Tool<PhaseName>      = "operate_<phasename>"
)
```

### Step 2: Implement Trade Tool Handler
Implement function tool helper returning soft errors inside `TradeResult`:

```go
type TradeResult struct {
	Status       string `json:"status"`
	Phase        string `json:"phase,omitempty"`
	Structure    string `json:"structure,omitempty"`
	Message      string `json:"message,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

func handle<PhaseName>(ctx agent.Context, args TradeArgs) (TradeResult, error) {
	if args.StructureType == "" {
		return TradeResult{Status: "error", ErrorMessage: "structure_type is required"}, nil
	}
	return TradeResult{
		Status:    "success",
		Phase:     Phase<PhaseName>,
		Structure: args.StructureType,
		Message:   fmt.Sprintf("Successfully executed %s operations on %s.", Phase<PhaseName>, args.StructureType),
	}, nil
}
```

### Step 3: Instantiate Trade Sub-Agent
In `NewTradeAgent`:
```go
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
```

### Step 4: Register in Construction Root Agent
In [`agents/construction/construction.go`](../../agents/construction/construction.go), instantiate and append the trade sub-agent:

```go
<phasename>Agent, err := NewTradeAgent(Trade<PhaseName>Name, Phase<PhaseName>, Tool<PhaseName>, m)
if err != nil {
	return nil, err
}

subAgents = append(subAgents, <phasename>Agent)
```

### Step 5: Verify Quality Gate
Run test suite to verify tool execution:
```bash
make check
```
