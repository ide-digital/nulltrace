package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appName = "nulltrace"

type Dirs struct {
	Config     string
	Data       string
	AuditLogs  string
	VaultDB    string
	DaemonLog  string
	ConfigFile string
	TokenFile  string
	Socket     string
}

func Resolve() (Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Dirs{}, fmt.Errorf("resolve home directory: %w", err)
	}
	var cfg, data string
	if runtime.GOOS == "windows" {
		if app := os.Getenv("AppData"); app != "" {
			cfg = filepath.Join(app, appName)
		} else {
			cfg = filepath.Join(home, "AppData", "Roaming", appName)
		}
		if local := os.Getenv("LocalAppData"); local != "" {
			data = filepath.Join(local, appName)
		} else {
			data = filepath.Join(home, "AppData", "Local", appName)
		}
	} else {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			cfg = filepath.Join(xdg, appName)
		} else {
			cfg = filepath.Join(home, ".config", appName)
		}
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			data = filepath.Join(xdg, appName)
		} else {
			data = filepath.Join(home, ".local", "share", appName)
		}
	}

	d := Dirs{
		Config:     cfg,
		Data:       data,
		AuditLogs:  filepath.Join(data, "audit_logs"),
		VaultDB:    filepath.Join(data, "vault.db"),
		DaemonLog:  filepath.Join(data, "daemon.log"),
		ConfigFile: filepath.Join(cfg, "config.yaml"),
		TokenFile:  filepath.Join(cfg, "daemon.token"),
		Socket:     filepath.Join(data, "nulltraced.sock"),
	}
	return d, nil
}

func (d Dirs) Ensure() error {
	for _, dir := range []string{d.Config, d.Data, d.AuditLogs} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}
