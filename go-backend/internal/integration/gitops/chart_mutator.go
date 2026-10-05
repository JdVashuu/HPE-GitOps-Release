package gitops

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reVersion    = regexp.MustCompile(`(?m)^version:\s*.+`)
	reAppVersion = regexp.MustCompile(`(?m)^appVersion:\s*.+`)
	reAnnotation = regexp.MustCompile(`(?m)^\s*recipe-detection/values-file:\s*\S+`)
)

// UpdateChartMetadata updates version, appVersion, and values-file annotations in Chart.yaml.
func UpdateChartMetadata(chartFilePath, version, valuesFileName string) error {
	contentBytes, err := os.ReadFile(chartFilePath)
	if err != nil {
		return fmt.Errorf("read Chart.yaml failed: %w", err)
	}

	content := string(contentBytes)
	content = reVersion.ReplaceAllString(content, "version: "+version)
	content = reAppVersion.ReplaceAllString(content, fmt.Sprintf(`appVersion: "%s"`, version))

	annotationLine := "  recipe-detection/values-file: " + valuesFileName
	if reAnnotation.MatchString(content) {
		content = reAnnotation.ReplaceAllString(content, strings.TrimSpace(annotationLine))
	} else if strings.Contains(content, "annotations:") {
		content = strings.Replace(content, "annotations:", "annotations:\n"+annotationLine, 1)
	} else {
		content = strings.TrimSpace(content) + "\nannotations:\n" + annotationLine + "\n"
	}

	return os.WriteFile(chartFilePath, []byte(content), 0644)
}
