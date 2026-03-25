package connector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/viant/scy"
)

func normalizeSecretResourceURL(resource *scy.Resource) error {
	if resource == nil || resource.URL == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to expand home directory for secret URL %q: %w", resource.URL, err)
	}

	switch {
	case strings.HasPrefix(resource.URL, "file://~/"):
		resource.URL = "file://" + filepath.ToSlash(filepath.Join(home, strings.TrimPrefix(resource.URL, "file://~/")))
	case isRelativeSecretURL(resource.URL):
		candidate := filepath.Join(home, ".secret", resource.URL)
		if filepath.Ext(candidate) == "" {
			if _, err := os.Stat(candidate + ".json"); err == nil {
				candidate += ".json"
			}
		}
		resource.URL = filepath.ToSlash(candidate)
	}
	return nil
}

func NormalizeSecretResourceURL(resource *scy.Resource) error {
	return normalizeSecretResourceURL(resource)
}

func isRelativeSecretURL(URL string) bool {
	if URL == "" {
		return false
	}
	if strings.Contains(URL, "://") {
		return false
	}
	return !filepath.IsAbs(URL) && !strings.HasPrefix(URL, "~")
}
