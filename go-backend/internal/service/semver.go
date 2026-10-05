package service

import (
	"strconv"
	"strings"
)

// NormalizeVersion trims whitespace and strips leading "v" or "V".
func NormalizeVersion(version string) string {
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// NextPatchVersion calculates the next available patch version given an existing set of versions.
func NextPatchVersion(current string, existing map[string]struct{}) string {
	candidate := bumpLastSegment(NormalizeVersion(current))
	for {
		if _, found := existing[candidate]; !found {
			return candidate
		}
		candidate = bumpLastSegment(candidate)
	}
}

func bumpLastSegment(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return version + ".1"
	}
	last := parts[len(parts)-1]
	if n, err := strconv.Atoi(last); err == nil {
		parts[len(parts)-1] = strconv.Itoa(n + 1)
		return strings.Join(parts, ".")
	}
	return version + ".1"
}
