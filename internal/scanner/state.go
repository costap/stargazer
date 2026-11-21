package scanner

import (
	"fmt"
	"os"

	"github.com/costap/stargazer/internal/domain"
	"gopkg.in/yaml.v3"
)

// LoadDispatchState reads the state file. The error wraps fs.ErrNotExist when the
// file is absent, so callers can tell "nothing recorded yet" from "unreadable".
func LoadDispatchState(filePath string) (*domain.DispatchState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read state file: %w", err)
	}
	var st domain.DispatchState
	if err := yaml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("yaml parse error in %s: %w", filePath, err)
	}
	return &st, nil
}
