package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// HaulerManifest represents a Rancher Hauler Images manifest.
// See https://github.com/rancher/hauler
type HaulerManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   HaulerMeta   `yaml:"metadata"`
	Spec       HaulerSpec   `yaml:"spec"`
}

type HaulerMeta struct {
	Name      string `yaml:"name"`
	CreatedAt string `yaml:"createdAt,omitempty"`
}

type HaulerSpec struct {
	Images []HaulerImage `yaml:"images"`
}

type HaulerImage struct {
	Name string `yaml:"name"`
}

// WriteHaulerManifest writes a Hauler-compatible Images manifest from image refs.
func WriteHaulerManifest(images []string, rancherVersion, outputPath string) error {
	if outputPath == "" {
		outputPath = rancherVersion + "-hauler.yaml"
	}
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}

	seen := make(map[string]struct{}, len(images))
	unique := make([]string, 0, len(images))
	for _, img := range images {
		img = strings.TrimSpace(img)
		if img == "" {
			continue
		}
		if _, ok := seen[img]; ok {
			continue
		}
		seen[img] = struct{}{}
		unique = append(unique, img)
	}
	sort.Strings(unique)

	entries := make([]HaulerImage, 0, len(unique))
	for _, img := range unique {
		entries = append(entries, HaulerImage{Name: img})
	}

	manifest := HaulerManifest{
		APIVersion: "content.hauler.cattle.io/v1",
		Kind:       "Images",
		Metadata: HaulerMeta{
			Name: strings.TrimPrefix(rancherVersion, "v") + "-rancher-images",
		},
		Spec: HaulerSpec{Images: entries},
	}

	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshal hauler manifest: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("write hauler manifest: %w", err)
	}
	return nil
}

// WriteImageList writes a newline-separated image list file.
func WriteImageList(images []string, outputPath string) error {
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}
	sort.Strings(images)
	content := strings.Join(images, "\n")
	if len(images) > 0 {
		content += "\n"
	}
	return os.WriteFile(outputPath, []byte(content), 0o644)
}
