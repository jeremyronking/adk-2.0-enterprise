package chatagent

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/v2/model"
)

type mockLLM struct{}

func (m mockLLM) Name() string { return "mock-model" }

func (m mockLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {}
}

func TestNewChatAgent(t *testing.T) {
	mM := mockLLM{}
	ag, err := New(mM)
	if err != nil {
		t.Fatalf("failed to create chat agent: %v", err)
	}
	if ag == nil {
		t.Fatal("expected non-nil root agent")
	}
	if ag.Name() != Name {
		t.Errorf("got agent name %q, want %q", ag.Name(), Name)
	}
}
