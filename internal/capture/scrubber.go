package capture

import (
	"regexp"
	"strings"
)

var (
	bearerRegex        = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9_\-\.]{20,}`)
	urlCredsRegex      = regexp.MustCompile(`(https?://)([^:\s]+:[^@\s]+)@`)
	awsKeyRegex        = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	ghTokenRegex       = regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{82}\b`)
	slackTokenRegex    = regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)
	privKeyRegex       = regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]*?-----END [A-Z ]+ PRIVATE KEY-----`)
	genericSecretRegex = regexp.MustCompile(`(?i)\b([a-z0-9_]*(?:api[_-]?key|secret|token|password|passwd|auth_key|auth_token)[a-z0-9_]*|\bauth\b)\s*([:=])\s*['"]?([^\s'"]+)['"]?`)
)

// Scrub scans a string for secrets, credentials, or tokens, replacing them with redaction placeholders.
// Returns the scrubbed string and a boolean indicating whether any secret was redacted.
func Scrub(s string) (string, bool) {
	orig := s

	s = privKeyRegex.ReplaceAllString(s, "[REDACTED_PRIVATE_KEY]")
	s = bearerRegex.ReplaceAllString(s, "${1}[REDACTED]")
	s = urlCredsRegex.ReplaceAllString(s, "${1}[REDACTED]@")
	s = awsKeyRegex.ReplaceAllString(s, "[REDACTED_AWS_KEY]")
	s = ghTokenRegex.ReplaceAllString(s, "[REDACTED_GITHUB_TOKEN]")
	s = slackTokenRegex.ReplaceAllString(s, "[REDACTED_SLACK_TOKEN]")

	s = genericSecretRegex.ReplaceAllStringFunc(s, func(match string) string {
		sub := genericSecretRegex.FindStringSubmatch(match)
		if len(sub) >= 4 {
			val := strings.Trim(sub[3], `'"`)
			if strings.HasPrefix(val, "[REDACTED") {
				return match
			}
			return sub[1] + sub[2] + "[REDACTED]"
		}
		return match
	})

	redacted := s != orig
	return s, redacted
}

// IsSensitive returns true if any credential or secret pattern is detected.
func IsSensitive(s string) bool {
	return bearerRegex.MatchString(s) ||
		urlCredsRegex.MatchString(s) ||
		awsKeyRegex.MatchString(s) ||
		ghTokenRegex.MatchString(s) ||
		slackTokenRegex.MatchString(s) ||
		privKeyRegex.MatchString(s) ||
		genericSecretRegex.MatchString(s)
}
