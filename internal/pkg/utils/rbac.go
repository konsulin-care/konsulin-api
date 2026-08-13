package utils

import (
	"net/url"
	"strings"
)

func PathMatch(requestPath, policyPath string) bool {
	requestURL, err := url.Parse(requestPath)
	if err != nil {
		return false
	}

	policyURL, err := url.Parse(policyPath)
	if err != nil {
		return false
	}
	if !strings.HasPrefix(requestURL.Path, policyURL.Path) {
		return false
	}
	if strings.HasSuffix(policyURL.Path, "/") {
		if requestURL.Path != policyURL.Path {
			return false
		}
	} else {
		if len(requestURL.Path) > len(policyURL.Path) {
			if requestURL.Path[len(policyURL.Path)] != '/' {
				return false
			}
		}
	}

	if len(policyURL.RawQuery) == 0 {
		return true
	}

	return requestURL.RawQuery == policyURL.RawQuery
}

func NormalizePath(rawURL string) string {
	path := strings.TrimPrefix(rawURL, "/")

	if !strings.HasPrefix(path, "fhir/") {
		path = "fhir/" + path
	}

	return "/" + path
}

func RequiresPatientOwnership(resourceType string) bool {
	return false
}

func RequiresPractitionerOwnership(resourceType string) bool {
	return false
}

func IsPublicResource(resourceType string) bool {
	return false
}

func ExtractResourceTypeFromPath(path string) string {
	u, err := url.Parse(path)
	if err != nil {

		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) >= 2 && strings.EqualFold(parts[0], "fhir") {
			return parts[1]
		} else if len(parts) >= 1 {
			return parts[0]
		}
		return ""
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	if len(parts) >= 2 && strings.EqualFold(parts[0], "fhir") {
		return parts[1]
	} else if len(parts) >= 1 {
		return parts[0]
	}

	return ""
}
