package service

import "testing"

func TestValidateUserAccountCredentialsRequiresAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]any
		wantError   bool
	}{
		{name: "missing", credentials: map[string]any{}, wantError: true},
		{name: "blank", credentials: map[string]any{"api_key": "  "}, wantError: true},
		{name: "present", credentials: map[string]any{"api_key": "sk-user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserAccountCredentials(AccountTypeAPIKey, map[string]any{"credentials": tt.credentials})
			if tt.wantError && err == nil {
				t.Fatal("expected invalid API key credentials to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("valid API key credentials were rejected: %v", err)
			}
		})
	}
}

func TestValidateUserAccountCredentialsChecksStructuredTypes(t *testing.T) {
	tests := []struct {
		name        string
		accountType string
		credentials map[string]any
		wantError   bool
	}{
		{name: "upstream missing URL", accountType: AccountTypeUpstream, credentials: map[string]any{"api_key": "key"}, wantError: true},
		{name: "upstream valid", accountType: AccountTypeUpstream, credentials: map[string]any{"api_key": "key", "base_url": "https://api.example.com"}},
		{name: "bedrock missing auth", accountType: AccountTypeBedrock, credentials: map[string]any{}, wantError: true},
		{name: "bedrock API key", accountType: AccountTypeBedrock, credentials: map[string]any{"api_key": "key"}},
		{name: "bedrock AWS keys", accountType: AccountTypeBedrock, credentials: map[string]any{"aws_access_key_id": "id", "aws_secret_access_key": "secret"}},
		{name: "service account missing", accountType: AccountTypeServiceAccount, credentials: map[string]any{}, wantError: true},
		{name: "service account JSON", accountType: AccountTypeServiceAccount, credentials: map[string]any{"service_account_json": "{}"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserAccountCredentials(tt.accountType, map[string]any{"credentials": tt.credentials})
			if tt.wantError && err == nil {
				t.Fatal("expected invalid credentials to be rejected")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("valid credentials were rejected: %v", err)
			}
		})
	}
}
