package service

import (
	"encoding/json"
	"testing"
)

func TestCreateOpenAITestPayloadUsesCustomPrompt(t *testing.T) {
	payload := createOpenAITestPayload("gpt-5.6", true, "  Reply with pong  ")
	input := payload["input"].([]map[string]any)
	content := input[0]["content"].([]map[string]any)
	if got := content[0]["text"]; got != "Reply with pong" {
		t.Fatalf("custom OpenAI test prompt was not preserved: %#v", got)
	}
}

func TestCreateTestPayloadUsesDefaultAndCustomPrompt(t *testing.T) {
	defaultPayload, err := createTestPayload("claude-sonnet-4-6", "")
	if err != nil {
		t.Fatalf("create default payload: %v", err)
	}
	if got := claudeTestPayloadPrompt(defaultPayload); got != "hi" {
		t.Fatalf("expected default prompt, got %q", got)
	}

	customPayload, err := createTestPayload("claude-sonnet-4-6", "Summarize this")
	if err != nil {
		t.Fatalf("create custom payload: %v", err)
	}
	if got := claudeTestPayloadPrompt(customPayload); got != "Summarize this" {
		t.Fatalf("expected custom prompt, got %q", got)
	}
}

func TestBuildGrokManualTestBodyUsesCustomPrompt(t *testing.T) {
	body, err := buildGrokManualTestBody("grok-4", "Reply with pong")
	if err != nil {
		t.Fatalf("build Grok test body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode Grok test body: %v", err)
	}
	if got := payload["input"]; got != "Reply with pong" {
		t.Fatalf("custom Grok test prompt was not preserved: %#v", got)
	}
}

func claudeTestPayloadPrompt(payload map[string]any) string {
	messages := payload["messages"].([]map[string]any)
	content := messages[0]["content"].([]map[string]any)
	prompt, _ := content[0]["text"].(string)
	return prompt
}
