package scanner

import (
	"fmt"
	"os"

	"github.com/costap/stargazer/internal/domain"
	"gopkg.in/yaml.v3"
)

func LoadRoadmapManifest(filePath string) (*domain.RoadmapManifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read roadmap manifest: %w", err)
	}

	var manifest domain.RoadmapManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("yaml parse error in %s: %w", filePath, err)
	}

	return &manifest, nil
}
