package broker

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed data/*.yaml
var dataFS embed.FS

type Registry struct {
	byID     map[string]Broker
	byDomain map[string]Broker
	all      []Broker
}

var (
	global     *Registry
	globalErr  error
	globalOnce sync.Once
)

type yamlFile struct {
	Brokers []Broker `yaml:"brokers"`
}

func Load() (*Registry, error) {
	entries := map[string]Broker{}
	err := fs.WalkDir(dataFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		raw, err := fs.ReadFile(dataFS, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var file yamlFile
		if err := yaml.Unmarshal(raw, &file); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, b := range file.Brokers {
			if b.ID == "" {
				return fmt.Errorf("%s: broker missing id", path)
			}
			b.ID = strings.ToLower(b.ID)
			if b.RepopulationPeriodDays == 0 {
				b.RepopulationPeriodDays = 60
			}
			if b.JurisdictionCoverage == "" {
				b.JurisdictionCoverage = JurisdictionAll
			}
			if existing, ok := entries[b.ID]; ok {
				entries[b.ID] = mergeBroker(existing, b)
			} else {
				entries[b.ID] = b
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	reg := &Registry{
		byID:     entries,
		byDomain: make(map[string]Broker, len(entries)),
		all:      make([]Broker, 0, len(entries)),
	}
	for _, b := range entries {
		reg.all = append(reg.all, b)
		if b.Domain != "" {
			reg.byDomain[strings.ToLower(b.Domain)] = b
		}
	}
	sort.Slice(reg.all, func(i, j int) bool {
		return strings.ToLower(reg.all[i].Name) < strings.ToLower(reg.all[j].Name)
	})
	return reg, nil
}

func mergeBroker(a, b Broker) Broker {
	out := a
	if b.Name != "" {
		out.Name = b.Name
	}
	if b.Domain != "" {
		out.Domain = b.Domain
	}
	if b.Category != "" {
		out.Category = b.Category
	}
	if b.Mechanism != "" {
		out.Mechanism = b.Mechanism
	}
	if b.OptOutURL != "" {
		out.OptOutURL = b.OptOutURL
	}
	if b.ContactEmail != "" {
		out.ContactEmail = b.ContactEmail
	}
	if b.JurisdictionCoverage != "" {
		out.JurisdictionCoverage = b.JurisdictionCoverage
	}
	if b.RepopulationPeriodDays != 0 {
		out.RepopulationPeriodDays = b.RepopulationPeriodDays
	}
	if b.Notes != "" {
		out.Notes = b.Notes
	}
	if b.Playbook != "" {
		out.Playbook = b.Playbook
	}
	out.RequiresCaptcha = out.RequiresCaptcha || b.RequiresCaptcha
	out.RequiresEmailConfirmation = out.RequiresEmailConfirmation || b.RequiresEmailConfirmation
	return out
}

func Default() (*Registry, error) {
	globalOnce.Do(func() {
		global, globalErr = Load()
	})
	return global, globalErr
}

func (r *Registry) Get(id string) (Broker, bool) {
	b, ok := r.byID[strings.ToLower(id)]
	return b, ok
}

func (r *Registry) ByDomain(domain string) (Broker, bool) {
	b, ok := r.byDomain[strings.ToLower(domain)]
	return b, ok
}

func (r *Registry) All() []Broker {
	return r.all
}

func (r *Registry) ByMechanism(m Mechanism) []Broker {
	var out []Broker
	for _, b := range r.all {
		if b.Mechanism == m || b.Mechanism == MechanismHybrid {
			out = append(out, b)
		}
	}
	return out
}

func (r *Registry) MatchURL(rawURL string) (Broker, bool) {
	low := strings.ToLower(rawURL)
	for _, b := range r.all {
		if b.Domain != "" && strings.Contains(low, strings.ToLower(b.Domain)) {
			return b, true
		}
	}
	return Broker{}, false
}

func (r *Registry) Len() int { return len(r.all) }
