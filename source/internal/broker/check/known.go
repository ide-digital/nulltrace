package check

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// StaleAfter is how long a known issue may sit in the file before the run
// starts failing over it.
const StaleAfter = 60 * 24 * time.Hour

const dateLayout = "2006-01-02"

// KnownIssue is one entry in known_issues.yaml: a failure we have looked at
// and are deliberately leaving alone for now.
type KnownIssue struct {
	ID     string `yaml:"id" json:"id"`
	Field  string `yaml:"field" json:"field"`
	Reason string `yaml:"reason" json:"reason"`
	Added  string `yaml:"added" json:"added"`
}

func (k KnownIssue) addedAt() (time.Time, error) {
	return time.Parse(dateLayout, k.Added)
}

func LoadKnown(path string) ([]KnownIssue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list []KnownIssue
	if err := yaml.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range list {
		k := &list[i]
		k.ID = strings.ToLower(strings.TrimSpace(k.ID))
		if k.ID == "" || k.Field == "" || k.Reason == "" {
			return nil, fmt.Errorf("%s: entry %d needs id, field and reason", path, i+1)
		}
		if _, err := k.addedAt(); err != nil {
			return nil, fmt.Errorf("%s: %s/%s: added must look like 2026-10-26", path, k.ID, k.Field)
		}
	}
	return list, nil
}
