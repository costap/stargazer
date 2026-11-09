package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/costap/stargazer/internal/domain"
)

func LoadSignalsPayload(ctx context.Context, source string) (*domain.SignalsPayload, error) {
	var data []byte
	var err error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 6 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed fetching signals from %s: %w", source, err)
		}
		defer resp.Body.Close()

		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed reading response body: %w", err)
		}
	} else if source != "" {
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed reading signals file: %w", err)
		}
	} else {
		return nil, nil
	}

	var payload domain.SignalsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed unmarshaling signals payload: %w", err)
	}

	return &payload, nil
}
