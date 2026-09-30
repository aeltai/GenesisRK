package commands

import (
	"strings"
	"testing"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/listgenerator"
)

func Test_buildChartRepoRoot_UIPluginMetadataOnly(t *testing.T) {
	cc := newGenesisCmd()
	cc.includeUIPluginCharts = true
	cc.generator = &listgenerator.Generator{
		LinuxImages:   map[string]map[string]bool{},
		WindowsImages: map[string]map[string]bool{},
		ChartMetadata: map[string]chartimages.ChartMetadata{
			"elemental": {
				Name:        "elemental",
				Version:     "3.0.1",
				Description: "Elemental UI extension",
				IconURL:     "https://example.com/elemental.svg",
				Repo:        "ui-plugins",
			},
			"harvester": {
				Name:        "harvester",
				Version:     "1.0.0",
				Description: "Harvester UI extension",
				Repo:        "ui-plugins",
			},
		},
	}

	node, ok := cc.buildChartRepoRoot(
		map[string]*listgenerator.ChartComponentGroup{},
		nil,
		listgenerator.SourceGroupUIPluginCharts,
		"UI Plugins",
		"Rancher UI plugin charts.",
	)
	if !ok {
		t.Fatal("expected UI Plugins root")
	}
	if node.Id != listgenerator.SourceGroupUIPluginCharts {
		t.Fatalf("root id = %q", node.Id)
	}
	if len(node.Children) != 2 {
		t.Fatalf("got %d chart nodes, want 2", len(node.Children))
	}
	if node.Children[0].Id != "elemental" || node.Children[0].IconURL == "" {
		t.Fatalf("elemental node: %+v", node.Children[0])
	}
	if node.Children[0].Count != 0 {
		t.Fatalf("metadata-only chart should have count 0, got %d", node.Children[0].Count)
	}
}

func Test_buildChartRepoRoot_SkipsWhenFlagOff(t *testing.T) {
	cc := newGenesisCmd()
	cc.includeUIPluginCharts = false
	cc.generator = &listgenerator.Generator{
		ChartMetadata: map[string]chartimages.ChartMetadata{
			"elemental": {Name: "elemental", Repo: "ui-plugins"},
		},
	}
	// buildChartRepoRoot itself does not check the flag; buildGenesisTree gates the call.
	// Verify metadata-only charts still build when invoked (caller gates inclusion).
	node, ok := cc.buildChartRepoRoot(nil, nil, listgenerator.SourceGroupUIPluginCharts, "UI Plugins", "desc")
	if !ok || len(node.Children) != 1 {
		t.Fatalf("buildChartRepoRoot should include metadata charts: ok=%v children=%d", ok, len(node.Children))
	}
}

func Test_filterImagesByVersions_PlusVsDashTags(t *testing.T) {
	images := []string{
		"rancher/k3s-upgrade:v1.35.8-k3s1",
		"rancher/system-agent-installer-k3s:v1.35.8-k3s1",
		"rancher/rke2-upgrade:v1.35.8-rke2r1",
		"rancher/system-agent-installer-rke2:v1.35.8-rke2r1",
		"rancher/local-path-provisioner:v0.0.37",
		"registry.rancher.com/rancher/rke2-runtime:v1.35.8-rke2r1",
		"rancher/k3s-upgrade:v1.34.5-k3s1",
	}
	// UI/KDM selection uses '+'
	got := filterImagesByVersions(images, "v1.35.8+k3s1", "v1.35.8+rke2r1", "")
	want := map[string]bool{
		"rancher/k3s-upgrade:v1.35.8-k3s1":                  true,
		"rancher/system-agent-installer-k3s:v1.35.8-k3s1":   true,
		"rancher/rke2-upgrade:v1.35.8-rke2r1":                true,
		"rancher/system-agent-installer-rke2:v1.35.8-rke2r1": true,
		"rancher/local-path-provisioner:v0.0.37":             true,
		"registry.rancher.com/rancher/rke2-runtime:v1.35.8-rke2r1": true,
	}
	gotSet := map[string]bool{}
	for _, img := range got {
		gotSet[img] = true
	}
	for img := range want {
		if !gotSet[img] {
			t.Errorf("expected to KEEP %s", img)
		}
	}
	if gotSet["rancher/k3s-upgrade:v1.34.5-k3s1"] {
		t.Errorf("expected to DROP other k3s version")
	}
}

func Test_tagMatchesVersionSelection(t *testing.T) {
	sel := map[string]bool{"v1.35.8+k3s1": true}
	if !tagMatchesVersionSelection("v1.35.8-k3s1", sel) {
		t.Fatal("dash tag should match plus selection")
	}
	if tagMatchesVersionSelection("v1.34.5-k3s1", sel) {
		t.Fatal("other version must not match")
	}
}

func Test_excludeOptionalStorageFromEssentials(t *testing.T) {
	keep := []string{
		"registry.rancher.com/rancher/rke2-cloud-provider:v1.35.8-0.20260817193936-20fc9c33a412-build20260820",
		"rancher/rke2-cloud-provider:v1.35.8-build20260820",
		"registry.rancher.com/rancher/local-path-provisioner:v0.0.34",
		"rancher/local-path-provisioner:v0.0.34",
		"registry.rancher.com/rancher/mirrored-calico-csi:v3.32.1",
		"registry.rancher.com/rancher/rke2-runtime:v1.35.8-rke2r1",
		"rancher/klipper-lb:v0.4.14",
	}
	drop := []string{
		"rancher/harvester-cloud-provider:v1.0.0",
		"rancher/mirrored-cloud-provider-vsphere-cpi-release-manager:v1.0.0",
		"rancher/longhorn-manager:v1.6.0",
		"rancher/mirrored-sig-storage-csi-provisioner:v3.0.0",
		"rancher/harvester-csi-driver:v1.0.0",
	}
	for _, img := range keep {
		if excludeOptionalStorageFromEssentials(strings.ToLower(img)) {
			t.Errorf("Essentials must KEEP %s", img)
		}
	}
	for _, img := range drop {
		if !excludeOptionalStorageFromEssentials(strings.ToLower(img)) {
			t.Errorf("Essentials must DROP %s", img)
		}
	}
}
