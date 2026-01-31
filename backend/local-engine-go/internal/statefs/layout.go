package statefs

import (
	"fmt"
	"path/filepath"
	"strings"
)

func baseDir(root, imageID string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("state store root is required")
	}
	engineID, version := parseImageID(imageID)
	return filepath.Join(root, "engines", engineID, version, "base"), nil
}

func statesDir(root, imageID string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("state store root is required")
	}
	engineID, version := parseImageID(imageID)
	return filepath.Join(root, "engines", engineID, version, "states"), nil
}

func stateDir(root, imageID, stateID string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("state store root is required")
	}
	if strings.TrimSpace(stateID) == "" {
		return "", fmt.Errorf("state id is required")
	}
	engineID, version := parseImageID(imageID)
	return filepath.Join(root, "engines", engineID, version, "states", stateID), nil
}

func jobRuntimeDir(root, jobID string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("state store root is required")
	}
	if strings.TrimSpace(jobID) == "" {
		return "", fmt.Errorf("job id is required")
	}
	return filepath.Join(root, "jobs", jobID, "runtime"), nil
}

func parseImageID(imageID string) (string, string) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return "unknown", "latest"
	}
	tag := ""
	digest := ""
	if at := strings.Index(imageID, "@"); at != -1 {
		if at+1 < len(imageID) {
			digest = imageID[at+1:]
		}
		imageID = imageID[:at]
	}
	if digest == "" {
		if colon := strings.LastIndex(imageID, ":"); colon != -1 && colon > strings.LastIndex(imageID, "/") {
			tag = imageID[colon+1:]
			imageID = imageID[:colon]
		}
	} else {
		tag = digest
	}
	engine := imageID
	if slash := strings.LastIndex(engine, "/"); slash != -1 {
		engine = engine[slash+1:]
	}
	engine = sanitizeSegment(engine)
	tag = sanitizeSegment(tag)
	if tag == "" {
		tag = "latest"
	}
	return engine, tag
}

func sanitizeSegment(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if r > 127 {
			b.WriteByte('_')
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := b.String()
	if out == "" || out == "." || out == ".." {
		return "unknown"
	}
	return out
}
