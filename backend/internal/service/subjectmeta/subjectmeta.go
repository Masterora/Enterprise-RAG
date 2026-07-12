package subjectmeta

import "strings"

func NormalizeVisibility(visibility string) string {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "public":
		return "public"
	default:
		return "private"
	}
}
