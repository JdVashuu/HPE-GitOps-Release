package gitops

import (
	"strings"

	"gopkg.in/yaml.v3"
	"hpe-recipe/internal/model"
)

// ResolveValuesFileName determines the values file name for a release.
func ResolveValuesFileName(release *model.HelmRelease) string {
	if release.ValuesFileName != "" {
		custom := strings.TrimSpace(strings.ReplaceAll(release.ValuesFileName, "\\", "/"))
		if !strings.Contains(custom, "..") && !strings.HasPrefix(custom, "/") {
			if !strings.HasSuffix(custom, ".yaml") {
				custom += ".yaml"
			}
			return custom
		}
	}
	return "values-v" + strings.TrimPrefix(strings.TrimPrefix(release.Version, "v"), "V") + ".yaml"
}

// GenerateValuesYAML produces the full values YAML content matching the Helm chart schema.
func GenerateValuesYAML(release *model.HelmRelease, valuesFileName string) (string, error) {
	relCopy := *release
	relCopy.ValuesFileName = valuesFileName

	wrapper := model.HelmReleaseWrapper{
		RecipeData: relCopy,
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
