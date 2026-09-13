package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultListenAddr = "127.0.0.1:7738"

// File is the YAML next to the vault. Passwords and API keys stay in the DB.
type File struct {
	ListenAddr   string          `yaml:"listen_addr"`
	SMTP         SMTPConfig      `yaml:"smtp"`
	IMAP         IMAPConfig      `yaml:"imap"`
	Proxy        string          `yaml:"proxy"`
	HIBP         HIBPConfig      `yaml:"hibp"`
	Browser      BrowserConfig   `yaml:"browser"`
	CapSolver    CapSolverConfig `yaml:"capsolver"`
	Footprint    FootprintConfig `yaml:"footprint"`
	Scheduler    SchedulerConfig `yaml:"scheduler"`
	Mailer       MailerConfig    `yaml:"mailer"`
	Jurisdiction string          `yaml:"jurisdiction"`
}

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	From     string `yaml:"from"`
	STARTTLS bool   `yaml:"starttls"`
}

type IMAPConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Username  string `yaml:"username"`
	Mailbox   string `yaml:"mailbox"`
	TLS       bool   `yaml:"tls"`
	AutoFetch bool   `yaml:"auto_fetch"`
}

type HIBPConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIBase string `yaml:"api_base"`
}

type BrowserConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Headless       bool   `yaml:"headless"`
	Stealth        bool   `yaml:"stealth"`
	Bin            string `yaml:"bin"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type CapSolverConfig struct {
	Enabled        bool   `yaml:"enabled"`
	APIBase        string `yaml:"api_base"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type FootprintConfig struct {
	Enabled        bool `yaml:"enabled"`
	MaxConcurrent  int  `yaml:"max_concurrent"`
	TimeoutSeconds int  `yaml:"timeout_seconds"`
}

type SchedulerConfig struct {
	RepopulationDays int `yaml:"repopulation_days"`
}

type MailerConfig struct {
	PerMinute int `yaml:"per_minute"`
}

func Default() File {
	return File{
		ListenAddr:   DefaultListenAddr,
		Jurisdiction: "CCPA",
		SMTP: SMTPConfig{
			Port:     587,
			STARTTLS: true,
		},
		IMAP: IMAPConfig{
			Port:    993,
			Mailbox: "INBOX",
			TLS:     true,
		},
		HIBP: HIBPConfig{
			Enabled: true,
			APIBase: "https://haveibeenpwned.com/api/v3",
		},
		Browser: BrowserConfig{
			Enabled:        false,
			Headless:       true,
			Stealth:        true,
			TimeoutSeconds: 45,
		},
		CapSolver: CapSolverConfig{
			Enabled:        false,
			APIBase:        "https://api.capsolver.com",
			TimeoutSeconds: 60,
		},
		Footprint: FootprintConfig{
			Enabled:        true,
			MaxConcurrent:  8,
			TimeoutSeconds: 15,
		},
		Scheduler: SchedulerConfig{RepopulationDays: 45},
		Mailer:    MailerConfig{PerMinute: 5},
	}
}

func Load(cfgPath string) (File, error) {
	cfg := Default()
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			if v := os.Getenv("NULLTRACE_LISTEN"); v != "" {
				cfg.ListenAddr = v
			}
			return cfg, nil
		}
		return File{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = DefaultListenAddr
	}
	if v := os.Getenv("NULLTRACE_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if cfg.Mailer.PerMinute <= 0 {
		cfg.Mailer.PerMinute = 5
	}
	if cfg.Scheduler.RepopulationDays <= 0 {
		cfg.Scheduler.RepopulationDays = 45
	}
	if cfg.IMAP.Mailbox == "" {
		cfg.IMAP.Mailbox = "INBOX"
	}
	if cfg.SMTP.Port == 0 {
		cfg.SMTP.Port = 587
	}
	if cfg.IMAP.Port == 0 {
		cfg.IMAP.Port = 993
	}
	if cfg.Browser.TimeoutSeconds <= 0 {
		cfg.Browser.TimeoutSeconds = 45
	}
	if cfg.CapSolver.APIBase == "" {
		cfg.CapSolver.APIBase = "https://api.capsolver.com"
	}
	if cfg.CapSolver.TimeoutSeconds <= 0 {
		cfg.CapSolver.TimeoutSeconds = 60
	}
	if cfg.Footprint.MaxConcurrent <= 0 {
		cfg.Footprint.MaxConcurrent = 8
	}
	if cfg.Footprint.TimeoutSeconds <= 0 {
		cfg.Footprint.TimeoutSeconds = 15
	}
	return cfg, nil
}

func Save(cfgPath string, cfg File) error {
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, cfgPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
