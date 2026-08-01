package timeagent

import (
	"context"
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

func (m mockAgentContext) Agent() agent.Agent                                      { return nil }
func (m mockAgentContext) Artifacts() agent.Artifacts                              { return nil }
func (m mockAgentContext) Memory() agent.Memory                                    { return nil }
func (m mockAgentContext) Session() session.Session                                { return nil }
func (m mockAgentContext) InvocationID() string                                    { return "" }
func (m mockAgentContext) Branch() string                                          { return "" }
func (m mockAgentContext) IsolationScope() string                                  { return "" }
func (m mockAgentContext) UserContent() *genai.Content                             { return nil }
func (m mockAgentContext) RunConfig() *agent.RunConfig                             { return nil }
func (m mockAgentContext) EndInvocation()                                          {}
func (m mockAgentContext) Ended() bool                                             { return false }
func (m mockAgentContext) ResumedInput(interruptID string) (any, bool)             { return nil, false }
func (m mockAgentContext) WithContext(ctx context.Context) agent.InvocationContext { return m }
func (m mockAgentContext) WithICDelta(d *agent.InvocationContextDelta) agent.InvocationContext {
	return m
}
func (m mockAgentContext) AgentName() string                    { return "" }
func (m mockAgentContext) ReadonlyState() session.ReadonlyState { return nil }
func (m mockAgentContext) UserID() string                       { return "" }
func (m mockAgentContext) AppName() string                      { return "" }
func (m mockAgentContext) SessionID() string                    { return "" }
func (m mockAgentContext) State() session.State                 { return nil }
func (m mockAgentContext) FunctionCallID() string               { return "" }
func (m mockAgentContext) Actions() *session.EventActions       { return nil }
func (m mockAgentContext) SearchMemory(ctx context.Context, query string) (*memory.SearchResponse, error) {
	return nil, nil
}
func (m mockAgentContext) ToolConfirmation() *toolconfirmation.ToolConfirmation { return nil }
func (m mockAgentContext) RequestConfirmation(hint string, payload any) error   { return nil }
func (m mockAgentContext) Path() string                                         { return "" }
func (m mockAgentContext) RunID() string                                        { return "" }
func (m mockAgentContext) SubScheduler() agent.DynamicSubScheduler              { return nil }
func (m mockAgentContext) WithAgentContext(ctx context.Context) agent.Context   { return m }
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

func TestNewTool(t *testing.T) {
	tTool, err := NewTool()
	if err != nil {
		t.Fatalf("failed to create time tool: %v", err)
	}
	if tTool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tTool.Name() != "get_current_time" {
		t.Errorf("got tool name %q, want %q", tTool.Name(), "get_current_time")
	}
}

func TestCurrentTime_ErrorHandling(t *testing.T) {
	mockCtx := mockAgentContext{Context: context.Background()}
	res, err := currentTime(mockCtx, LocationArgs{Location: ""})
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if res.Status != "error" {
		t.Errorf("expected status 'error', got %q", res.Status)
	}
	if res.ErrorMessage == "" {
		t.Error("expected non-empty error message")
	}
}
