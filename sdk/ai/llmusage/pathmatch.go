package llmusage

import "strings"

// pathsMatch reports whether a request path is covered by a resource pattern.
// A pattern may end in a wildcard to cover everything beneath a prefix. A
// specific pattern never matches a wildcard request path, so catch-all routes
// do not pick up resource-specific configuration.
func pathsMatch(requestPath, pattern string) bool {
	if pattern == "/*" {
		return true
	}
	if requestPath == pattern {
		return true
	}
	if strings.Contains(pattern, "*") {
		prefix := pattern[:strings.LastIndex(pattern, "*")]
		return strings.HasPrefix(requestPath, prefix)
	}
	return false
}

// preferMoreSpecificPath reports whether candidate is a better match than
// current: a pattern without a wildcard beats one with a wildcard, and a longer
// pattern beats a shorter one.
func preferMoreSpecificPath(candidate, current string) bool {
	candidateHasWildcard := strings.Contains(candidate, "*")
	currentHasWildcard := strings.Contains(current, "*")

	if !candidateHasWildcard && currentHasWildcard {
		return true
	}
	if candidateHasWildcard && !currentHasWildcard {
		return false
	}

	return len(candidate) > len(current)
}
