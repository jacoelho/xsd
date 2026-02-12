package preprocessor

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

func resolveSystemID(baseSystemID, schemaLocation string) (string, error) {
	if strings.Contains(schemaLocation, "\\") {
		return "", fmt.Errorf("schema location contains backslash: %q", schemaLocation)
	}
	if strings.HasPrefix(schemaLocation, "/") {
		return "", fmt.Errorf("schema location must be relative: %q", schemaLocation)
	}
	if schemaLocation == "" {
		return "", fmt.Errorf("schema location is empty")
	}
	if baseSystemID != "" && strings.Contains(baseSystemID, "\\") {
		return "", fmt.Errorf("base system ID contains backslash: %q", baseSystemID)
	}
	segments := strings.Split(schemaLocation, "/")
	if len(segments) == 0 {
		return "", fmt.Errorf("schema location is empty")
	}
	if slices.Contains(segments, "") {
		return "", fmt.Errorf("invalid schema location segment: %q", schemaLocation)
	}
	canonical := path.Clean(schemaLocation)
	if canonical == "." {
		return "", fmt.Errorf("schema location is empty")
	}
	if baseSystemID == "" {
		if strings.HasPrefix(canonical, "../") || canonical == ".." {
			return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
		}
		return canonical, nil
	}
	baseDir := baseDirSystemID(baseSystemID)
	if baseDir == "" {
		if strings.HasPrefix(canonical, "../") || canonical == ".." {
			return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
		}
		return canonical, nil
	}
	joined := path.Clean(baseDir + "/" + schemaLocation)
	if joined == "." {
		return "", fmt.Errorf("schema location is empty")
	}
	if strings.HasPrefix(joined, "../") || joined == ".." {
		return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
	}
	return joined, nil
}

func baseDirSystemID(systemID string) string {
	if systemID == "" {
		return ""
	}
	if strings.Contains(systemID, "\\") {
		return ""
	}
	idx := strings.LastIndex(systemID, "/")
	if idx == -1 {
		return ""
	}
	return systemID[:idx]
}
