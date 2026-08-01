// Package chatagent provides the root master router agent. It dynamically
// discovers sub-agents from the internal/registry package and routes tasks
// via ADK SubAgents transfer.
package chatagent

import (
	"fmt"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/geminitool"
	"google.golang.org/genai"

	// Import sub-agent packages for blank side-effect registration in registry
	_ "my-agent-enterprise/agents/construction"
	_ "my-agent-enterprise/agents/timeagent"
	_ "my-agent-enterprise/agents/weatheragent"
	"my-agent-enterprise/internal/registry"
)

// Name is the root agent's name.
const Name = "chat_agent"

// New builds the root master router agent, discovering registered sub-agents dynamically.
func New(m model.LLM) (agent.Agent, error) {
	subAgents, err := registry.All(m)
	if err != nil {
		return nil, fmt.Errorf("loading sub-agents from registry: %w", err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        Name,
		Model:       m,
		Description: "Master router assistant that routes user requests to registered sub-agents.",
		InstructionProvider: func(ctx agent.ReadonlyContext) (string, error) {
			var sb strings.Builder
			sb.WriteString("You are a master router assistant. Dynamically transfer user requests to the appropriate sub-agent based on their capabilities:\n\n")
			for _, spec := range registry.Specs() {
				fmt.Fprintf(&sb, "- %s: %s\n", spec.Name, spec.Description)
			}
			sb.WriteString("\nFall back to Google Search only for questions none of the sub-agents can answer.")
			return sb.String(), nil
		},
		SubAgents: subAgents,
		Tools: []tool.Tool{
			geminitool.GoogleSearch{},
		},
		GenerateContentConfig: &genai.GenerateContentConfig{
			ToolConfig: &genai.ToolConfig{
				IncludeServerSideToolInvocations: genai.Ptr(true),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", Name, err)
	}
	return a, nil
}
