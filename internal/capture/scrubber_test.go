package capture_test

import (
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/capture"
)

func TestScrubber_Scrub(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantRedacted bool
		wantContains string
	}{
		{
			name:         "plain text without secrets",
			input:        "git commit -m 'fix: typo in README'",
			wantRedacted: false,
			wantContains: "git commit -m 'fix: typo in README'",
		},
		{
			name:         "bearer token",
			input:        "curl -H 'Authorization: Bearer ya29.a0AfH6SMBabc1234567890xyz' https://api.example.com",
			wantRedacted: true,
			wantContains: "Bearer [REDACTED]",
		},
		{
			name:         "generic API key",
			input:        "export OPENAI_API_KEY=sk-proj-1234567890abcdefgh",
			wantRedacted: true,
			wantContains: "OPENAI_API_KEY=[REDACTED]",
		},
		{
			name:         "AWS secret access key",
			input:        "AWS_SECRET_ACCESS_KEY='wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'",
			wantRedacted: true,
			wantContains: "AWS_SECRET_ACCESS_KEY=[REDACTED]",
		},
		{
			name:         "AWS access key ID",
			input:        "access_key = AKIAIOSFODNN7EXAMPLE",
			wantRedacted: true,
			wantContains: "[REDACTED_AWS_KEY]",
		},
		{
			name:         "GitHub personal access token",
			input:        "git clone https://ghp_1234567890abcdef1234567890abcdef1234@github.com/repo.git",
			wantRedacted: true,
			wantContains: "[REDACTED_GITHUB_TOKEN]",
		},
		{
			name:         "Slack token",
			input:        "SLACK_BOT_TOKEN=xoxb-1234567890-abcdef12345",
			wantRedacted: true,
			wantContains: "[REDACTED_SLACK_TOKEN]",
		},
		{
			name:         "URL embedded credentials",
			input:        "git remote add origin https://admin:SuperSecret123@gitlab.com/group/repo.git",
			wantRedacted: true,
			wantContains: "https://[REDACTED]@",
		},
		{
			name: "private key block",
			input: "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0\n-----END RSA PRIVATE KEY-----",
			wantRedacted: true,
			wantContains: "[REDACTED_PRIVATE_KEY]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, redacted := capture.Scrub(tc.input)
			if redacted != tc.wantRedacted {
				t.Errorf("expected redacted=%v, got %v for %q", tc.wantRedacted, redacted, tc.input)
			}
			if !strings.Contains(got, tc.wantContains) {
				t.Errorf("expected scrubbed output to contain %q, got %q", tc.wantContains, got)
			}
			if tc.wantRedacted && !capture.IsSensitive(tc.input) {
				t.Errorf("IsSensitive returned false for sensitive input %q", tc.input)
			}
		})
	}
}
