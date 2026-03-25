package connector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/viant/scy"
)

func TestNormalizeSecretResourceURL(t *testing.T) {
	resource := scy.NewResource("", "file://~/.secret/mysql-prod.json", "blowfish://default")

	err := NormalizeSecretResourceURL(resource)
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	require.Equal(t, "file://"+filepath.ToSlash(filepath.Join(home, ".secret/mysql-prod.json")), resource.URL)
}

func TestNormalizeSecretResourceURLRelativeName(t *testing.T) {
	resource := scy.NewResource("", "mysql-prod", "blowfish://default")

	err := NormalizeSecretResourceURL(resource)
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	expected := filepath.ToSlash(filepath.Join(home, ".secret/mysql-prod"))
	if _, err := os.Stat(expected + ".json"); err == nil {
		expected += ".json"
	}
	require.Equal(t, expected, resource.URL)
}
