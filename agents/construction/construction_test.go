package construction

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
)

type mockAgentContext struct {
	context.Context
}

func (m mockAgentContext) Agent() agent.Agent                                    { return nil }
func (m mockAgentContext) Artifacts() agent.Artifacts                           { return nil }
func (m mockAgentContext) Memory() agent.Memory                                 { return nil }
func (m mockAgentContext) Session() session.Session                             { return nil }
func (m mockAgentContext) InvocationID() string                                 { return "" }
func (m mockAgentContext) Branch() string                                       { return "" }
func (m mockAgentContext) IsolationScope() string                               { return "" }
func (m mockAgentContext) UserContent() *genai.Content                          { return nil }
func (m mockAgentContext) RunConfig() *agent.RunConfig                          { return nil }
func (m mockAgentContext) EndInvocation()                                       {}
func (m mockAgentContext) Ended() bool                                          { return false }
func (m mockAgentContext) ResumedInput(interruptID string) (any, bool)          { return nil, false }
func (m mockAgentContext) WithContext(ctx context.Context) agent.InvocationContext { return m }
func (m mockAgentContext) WithICDelta(d *agent.InvocationContextDelta) agent.InvocationContext {
	return m
}
func (m mockAgentContext) AgentName() string                        { return "" }
func (m mockAgentContext) ReadonlyState() session.ReadonlyState    { return nil }
func (m mockAgentContext) UserID() string                           { return "" }
func (m mockAgentContext) AppName() string                          { return "" }
func (m mockAgentContext) SessionID() string                        { return "" }
func (m mockAgentContext) State() session.State                     { return nil }
func (m mockAgentContext) FunctionCallID() string                   { return "" }
func (m mockAgentContext) Actions() *session.EventActions           { return nil }
func (m mockAgentContext) SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error) {
	return nil, nil
}
func (m mockAgentContext) ToolConfirmation() *toolconfirmation.ToolConfirmation { return nil }
func (m mockAgentContext) RequestConfirmation(hint string, payload any) error    { return nil }
func (m mockAgentContext) Path() string                             { return "" }
func (m mockAgentContext) RunID() string                            { return "" }
func (m mockAgentContext) SubScheduler() agent.DynamicSubScheduler  { return nil }
func (m mockAgentContext) WithAgentContext(ctx context.Context) agent.Context { return m }
func (m mockAgentContext) WithAgentTimeout(timeout time.Duration) (agent.Context, context.CancelFunc) {
	return m, func() {}
}
func (m mockAgentContext) WithAgentCancel() (agent.Context, context.CancelFunc) {
	return m, func() {}
}
func (m mockAgentContext) OutputForAncestors() []string { return nil }
func (m mockAgentContext) WithDelta(d *agent.CommonContextDelta) agent.Context {
	return m
}

func TestInspectPhase(t *testing.T) {
	mockCtx := mockAgentContext{Context: context.Background()}
	res, err := inspectPhase(mockCtx, InspectArgs{Phase: "foundation"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "PASS" && res.Status != "FIX" {
		t.Errorf("unexpected status %q", res.Status)
	}
	if !strings.Contains(res.Findings, "foundation") {
		t.Errorf("findings %q missing phase name", res.Findings)
	}
}

func TestExecuteTrade(t *testing.T) {
	t.Run("construct action", func(t *testing.T) {
		res, err := executeTrade("framing", TradeArgs{StructureType: "mansion"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "success" {
			t.Errorf("got status %q, want 'success'", res.Status)
		}
		if !strings.Contains(res.Message, "Starting framing construction on mansion") {
			t.Errorf("unexpected message: %q", res.Message)
		}
	})

	t.Run("repair action", func(t *testing.T) {
		res, err := executeTrade("roofing", TradeArgs{StructureType: "shed", Action: "repair"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Message, "Performing repairs on roofing for shed") {
			t.Errorf("unexpected message: %q", res.Message)
		}
	})
}
