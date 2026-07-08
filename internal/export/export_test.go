package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aeltai/hangar-genesis/internal/export"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestWriteHaulerManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "test-hauler.yaml")
	images := []string{
		"docker.io/rancher/rancher:v2.13.1",
		"docker.io/rancher/rancher-agent:v2.13.1",
		"docker.io/rancher/rancher:v2.13.1", // duplicate
	}
	err := export.WriteHaulerManifest(images, "v2.13.1", out)
	require.NoError(t, err)

	data, err := os.ReadFile(out)
	require.NoError(t, err)

	var manifest export.HaulerManifest
	require.NoError(t, yaml.Unmarshal(data, &manifest))

	assert.Equal(t, "content.hauler.cattle.io/v1", manifest.APIVersion)
	assert.Equal(t, "Images", manifest.Kind)
	assert.Equal(t, "2.13.1-rancher-images", manifest.Metadata.Name)
	assert.Len(t, manifest.Spec.Images, 2)
	names := []string{manifest.Spec.Images[0].Name, manifest.Spec.Images[1].Name}
	assert.Contains(t, names, "docker.io/rancher/rancher:v2.13.1")
	assert.Contains(t, names, "docker.io/rancher/rancher-agent:v2.13.1")
}

func TestWriteImageList(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "images.txt")
	images := []string{"b.io/a:1", "a.io/b:2"}
	require.NoError(t, export.WriteImageList(images, out))

	content, err := os.ReadFile(out)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Equal(t, []string{"a.io/b:2", "b.io/a:1"}, lines)
}
