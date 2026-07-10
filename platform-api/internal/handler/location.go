package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// setLocation sets the Location response header to the path-absolute URL of a
// newly created resource under the API base path. It is a no-op when any
// segment is empty so a missing identifier never yields a malformed URL.
func setLocation(w http.ResponseWriter, segments ...string) {
	var b strings.Builder
	b.WriteString(constants.APIBasePath)
	for _, s := range segments {
		if s == "" {
			return
		}
		b.WriteString("/")
		b.WriteString(url.PathEscape(s))
	}
	w.Header().Set("Location", b.String())
}

// strOrEmpty returns the dereferenced string, or "" for nil.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
