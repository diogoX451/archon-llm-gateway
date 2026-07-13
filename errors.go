package llmgateway

import "strings"

func is429(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "429") || strings.Contains(s, "rate limit") || strings.Contains(s, "too many requests")
}

func is5xx(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "500") || strings.Contains(s, "502") ||
		strings.Contains(s, "503") || strings.Contains(s, "504") ||
		strings.Contains(s, "5xx") || strings.Contains(s, "unavailable")
}

func isRetryable(err error) bool {
	return is429(err) || is5xx(err) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "timeout"))
}
