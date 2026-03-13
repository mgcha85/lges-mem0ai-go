package prompts

import (
	"strings"
	"testing"
	"time"
)

func TestUserMemoryExtractionPrompt_ContainsDate(t *testing.T) {
	prompt := UserMemoryExtractionPrompt()
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(prompt, today) {
		t.Errorf("Prompt should contain today's date %s", today)
	}
}

func TestUserMemoryExtractionPrompt_ContainsKeyDirectives(t *testing.T) {
	prompt := UserMemoryExtractionPrompt()

	keywords := []string{
		"GENERATE FACTS SOLELY BASED ON THE USER'S MESSAGES",
		"Personal Information Organizer",
		`"facts"`,
		"json",
	}

	for _, kw := range keywords {
		if !strings.Contains(prompt, kw) {
			t.Errorf("Prompt should contain %q", kw)
		}
	}
}

func TestDefaultUpdateMemoryPrompt_ContainsActions(t *testing.T) {
	actions := []string{"ADD", "UPDATE", "DELETE", "NONE"}
	for _, action := range actions {
		if !strings.Contains(DefaultUpdateMemoryPrompt, action) {
			t.Errorf("Update memory prompt should contain action %q", action)
		}
	}
}

func TestGetUpdateMemoryMessages_EmptyMemory(t *testing.T) {
	result := GetUpdateMemoryMessages(nil, []string{"Name is John"})
	if !strings.Contains(result, "Current memory is empty") {
		t.Error("Expected 'Current memory is empty' for nil oldMemory")
	}
	if !strings.Contains(result, "Name is John") {
		t.Error("Expected new facts in the prompt")
	}
}

func TestGetUpdateMemoryMessages_WithExistingMemory(t *testing.T) {
	oldMemory := []map[string]string{
		{"id": "0", "text": "User is an engineer"},
	}
	result := GetUpdateMemoryMessages(oldMemory, []string{"Name is John"})
	if strings.Contains(result, "Current memory is empty") {
		t.Error("Should not say 'Current memory is empty' when memory exists")
	}
	if !strings.Contains(result, "User is an engineer") {
		t.Error("Expected old memory content in the prompt")
	}
	if !strings.Contains(result, "Name is John") {
		t.Error("Expected new facts in the prompt")
	}
}
