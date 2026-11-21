package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/costap/stargazer/internal/domain"
)

var (
	candidateFile  = regexp.MustCompile(`^(\d{3,})-.*\.md$`)
	candidateField = regexp.MustCompile(`^- \*\*([A-Za-z ]+):\*\*\s*(.*)$`)
)

// ScanCandidates reads NNN-*.md files in dir and returns the open ones, oldest first.
// A file is open only when its "Status" field says "open"; promoted, dropped and
// anything else is closed. A file with no Status field is named in the error rather
// than guessed at. A missing or unreadable dir is an error.
func ScanCandidates(dir string) ([]domain.Candidate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to read candidates dir: %w", err)
	}
	var open []domain.Candidate
	var bad []string
	for _, e := range entries {
		m := candidateFile.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		c, status, err := parseCandidate(filepath.Join(dir, e.Name()), m[1])
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", e.Name(), err))
			continue
		}
		if status == "" {
			bad = append(bad, e.Name()+": no Status field")
			continue
		}
		if strings.EqualFold(status, "open") {
			open = append(open, c)
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].Raised != open[j].Raised {
			if open[i].Raised == "" || open[j].Raised == "" {
				return open[j].Raised == ""
			}
			return open[i].Raised < open[j].Raised
		}
		return open[i].ID < open[j].ID
	})
	if len(bad) > 0 {
		return open, fmt.Errorf("candidates not readable: %s", strings.Join(bad, "; "))
	}
	return open, nil
}

func parseCandidate(path, id string) (domain.Candidate, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return domain.Candidate{}, "", err
	}
	defer f.Close()
	c := domain.Candidate{ID: id}
	status := ""
	first := true
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if first {
			first = false
			t := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if i := strings.Index(t, "\u2014"); i >= 0 {
				t = strings.TrimSpace(t[i+len("\u2014"):])
			}
			c.Title = t
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break
		}
		if m := candidateField.FindStringSubmatch(line); m != nil {
			switch strings.ToLower(strings.TrimSpace(m[1])) {
			case "status":
				status = strings.TrimSpace(m[2])
			case "raised":
				c.Raised = strings.TrimSpace(m[2])
			case "source":
				c.Source = strings.TrimSpace(m[2])
			}
		}
	}
	return c, status, sc.Err()
}
