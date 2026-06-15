package sp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type systemSettings struct {
	ScanPaths []string `json:"scan_paths"`
}

func loadSystemSettings() (systemSettings, error) {
	path, err := systemSettingsPath()
	if err != nil {
		return systemSettings{}, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return systemSettings{}, nil
	}
	if err != nil {
		return systemSettings{}, err
	}

	var settings systemSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return systemSettings{}, err
	}
	settings.ScanPaths = normalizeScanPaths(settings.ScanPaths)
	return settings, nil
}

func saveSystemSettings(settings systemSettings) error {
	path, err := systemSettingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	settings.ScanPaths = normalizeScanPaths(settings.ScanPaths)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func addSystemScanPath(path string) (systemSettings, string, error) {
	normalized, err := normalizeScanPath(path)
	if err != nil {
		return systemSettings{}, "", err
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return systemSettings{}, "", err
	}
	if !info.IsDir() {
		return systemSettings{}, "", fmt.Errorf("%s is not a directory", normalized)
	}

	settings, err := loadSystemSettings()
	if err != nil {
		return systemSettings{}, "", err
	}
	for _, existing := range settings.ScanPaths {
		if existing == normalized {
			return settings, normalized, nil
		}
	}

	settings.ScanPaths = append(settings.ScanPaths, normalized)
	if err := saveSystemSettings(settings); err != nil {
		return systemSettings{}, "", err
	}
	return settings, normalized, nil
}

func systemSettingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sp", "settings.json"), nil
}

func normalizeScanPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		cleaned, err := normalizeScanPath(path)
		if err != nil || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		normalized = append(normalized, cleaned)
	}
	return normalized
}

func normalizeScanPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("scan path cannot be empty")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
