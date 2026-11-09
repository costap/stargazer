package scanner

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/costap/stargazer/internal/config"
	"github.com/costap/stargazer/internal/domain"
)

var (
	// Extracts digits from filenames like "0001-record-architecture-decisions.md" or "1.md"
	fileNumRegex = regexp.MustCompile(`(\d+)`)

	// Matches title lines: "# 1. Title", "# 0001 - Title", or "# Title"
	headingRegex = regexp.MustCompile(`^#\s+(?:(?:\d+)[\.\-_ ]\s*)?(.*)$`)

	// Matches inline date: "Date: 2026-02-18" or "**Date:** 2026-02-18"
	dateRegex = regexp.MustCompile(`(?i)^\*?\*?Date:\*?\*?\s*([0-9]{4}-[0-9]{2}-[0-9]{2})`)
)

// ScanAllADRGroups walks all configured track directories (e.g. 2-product, 3-platform).
//
// It returns every ADR it could read together with an error joining one entry per
// group that failed (see errors.Join). The two are not exclusive: callers decide
// whether partial results are acceptable. A group that failed is never skipped silently.
func ScanAllADRGroups(groups []config.ADRGroupConfig) ([]domain.ADR, error) {
	var all []domain.ADR
	var errs []error

	for _, g := range groups {
		items, err := ScanADRDir(g.Namespace, g.Dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", g.Namespace, err))
			continue
		}
		all = append(all, items...)
	}

	// Sort deterministically: Track first, then numeric ADR Number
	sort.Slice(all, func(i, j int) bool {
		if all[i].Track == all[j].Track {
			return all[i].Number < all[j].Number
		}
		return all[i].Track < all[j].Track
	})

	return all, errors.Join(errs...)
}

// ScanADRDir scans a single directory of ADR markdown files.
func ScanADRDir(track, dir string) ([]domain.ADR, error) {
	var items []domain.ADR

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		item, err := ParseADRFile(track, fullPath, entry.Name())
		if err != nil {
			continue
		}

		item.LastCommit = getGitLastCommit(fullPath)
		items = append(items, item)
	}

	return items, nil
}

// ParseADRFile parses a single Nygard-style ADR document.
func ParseADRFile(track, fullPath, filename string) (domain.ADR, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return domain.ADR{}, err
	}
	defer file.Close()

	// 1. Resolve numeric ID from filename or fallback
	rawNum := "0"
	if m := fileNumRegex.FindString(filename); m != "" {
		rawNum = m
	}
	num, _ := strconv.Atoi(rawNum)
	paddedID := fmt.Sprintf("%04d", num)

	item := domain.ADR{
		Track:       track,
		Number:      num,
		RawID:       paddedID,
		QualifiedID: fmt.Sprintf("%s/%s", track, paddedID),
		Filename:    filename,
		Path:        fullPath,
		Status:      domain.StatusProposed, // default until parsed
	}

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Header # 1. Title
		if item.Title == "" && strings.HasPrefix(line, "# ") {
			if m := headingRegex.FindStringSubmatch(line); len(m) > 1 {
				item.Title = strings.TrimSpace(m[1])
			} else {
				item.Title = strings.TrimPrefix(line, "# ")
			}
			continue
		}

		// Inline Date metadata: "Date: 2026-02-18"
		if m := dateRegex.FindStringSubmatch(line); len(m) > 1 {
			if item.Date == "" {
				item.Date = m[1]
			}
			continue
		}

		// Section Headers: ## Status, ## Context, ## Decision, ## Consequences
		if strings.HasPrefix(line, "## ") {
			currentSection = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			continue
		}

		// Parse Section Body
		switch currentSection {
		case "status":
			if item.StatusLine == "" {
				item.StatusLine = line
			}
			statusLine := strings.ToLower(line)
			switch {
			case strings.HasPrefix(statusLine, "accepted"):
				item.Status = domain.StatusAccepted
			case strings.HasPrefix(statusLine, "rejected"):
				item.Status = domain.StatusRejected
			case strings.HasPrefix(statusLine, "superseded"):
				item.Status = domain.StatusSuperseded
			case strings.HasPrefix(statusLine, "proposed"):
				item.Status = domain.StatusProposed
			}
		}
	}

	if item.Title == "" {
		item.Title = strings.TrimSuffix(filename, ".md")
	}

	return item, scanner.Err()
}

// getGitLastCommit returns relative time of the last commit for a file
func getGitLastCommit(filePath string) string {
	cmd := exec.Command("git", "log", "-1", "--format=%cr", "--", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "uncommitted"
	}

	res := strings.TrimSpace(out.String())
	if res == "" {
		return "local changes"
	}
	return res
}
