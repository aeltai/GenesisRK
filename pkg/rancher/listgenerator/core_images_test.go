package listgenerator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
)

const testRancherImagesTxt = `# official rancher-images.txt (excerpt)
rancher/rancher:v2.13.1
rancher/rancher-agent:v2.13.1
rancher/shell:v0.5.1
rancher/machine:v0.15.0-rancher134
rancher/system-agent:v0.3.14-suc
rancher/kubectl:v1.34.1
rancher/fleet:v0.14.1
rancher/hardened-calico:v3.31.3-build20251210
quay.io/skopeo/stable:v1.19
`

func Test_generateCoreImagesFromRancherImagesTxt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testRancherImagesTxt)
	}))
	defer srv.Close()

	g, err := NewGenerator(&GeneratorOption{
		RancherVersion:      "v2.13.1",
		RancherImagesTxtURL: srv.URL,
		ChartsPaths:         map[string]chartimages.ChartRepoType{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.generateCoreImagesFromRancherImagesTxt(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Only allowlisted core images must be merged; chart/KDM-covered images
	// (fleet, calico) and third-party images must NOT come from this source.
	wantMerged := []string{
		"rancher/rancher:v2.13.1",
		"rancher/rancher-agent:v2.13.1",
		"rancher/shell:v0.5.1",
		"rancher/machine:v0.15.0-rancher134",
		"rancher/system-agent:v0.3.14-suc",
		"rancher/kubectl:v1.34.1",
	}
	wantSkipped := []string{
		"rancher/fleet:v0.14.1",
		"rancher/hardened-calico:v3.31.3-build20251210",
		"quay.io/skopeo/stable:v1.19",
	}
	for _, img := range wantMerged {
		sources, ok := g.LinuxImages[img]
		if !ok {
			t.Errorf("core image %q not merged", img)
			continue
		}
		if !sources[RancherCoreImagesSource] {
			t.Errorf("core image %q missing source tag %q, got %v",
				img, RancherCoreImagesSource, sources)
		}
	}
	for _, img := range wantSkipped {
		if _, ok := g.LinuxImages[img]; ok {
			t.Errorf("image %q must not be merged (not in allowlist)", img)
		}
	}
	if len(g.LinuxImages) != len(wantMerged) {
		t.Errorf("expected %d merged images, got %d: %v",
			len(wantMerged), len(g.LinuxImages), g.LinuxImages)
	}
}

func Test_generateCoreImagesFromRancherImagesTxt_Disabled(t *testing.T) {
	g, err := NewGenerator(&GeneratorOption{
		RancherVersion: "v2.13.1",
		ChartsPaths:    map[string]chartimages.ChartRepoType{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.generateCoreImagesFromRancherImagesTxt(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(g.LinuxImages) != 0 {
		t.Errorf("no URL configured: expected no merged images, got %v", g.LinuxImages)
	}
}

func Test_generateCoreImagesFromRancherImagesTxt_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	g, err := NewGenerator(&GeneratorOption{
		RancherVersion:      "v2.99.0",
		RancherImagesTxtURL: srv.URL,
		ChartsPaths:         map[string]chartimages.ChartRepoType{},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Missing release asset must surface as an error (callers treat it as
	// non-fatal and log a warning).
	if err := g.generateCoreImagesFromRancherImagesTxt(context.Background()); err == nil {
		t.Error("expected error for 404 response")
	}
}

func Test_generateFromPrimeRancherImages_AllowlistOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rancher/v2.14.6/rancher-images.txt") {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, testRancherImagesTxt)
	}))
	defer srv.Close()

	g, err := NewGenerator(&GeneratorOption{
		RancherVersion:   "v2.14.6",
		ImageListBaseURL: srv.URL,
		ChartsPaths:      map[string]chartimages.ChartRepoType{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.generateFromPrimeRancherImages(context.Background()); err != nil {
		t.Fatal(err)
	}

	wantSkipped := []string{
		"rancher/fleet:v0.14.1",
		"rancher/hardened-calico:v3.31.3-build20251210",
		"quay.io/skopeo/stable:v1.19",
	}
	for _, img := range wantSkipped {
		if _, ok := g.LinuxImages[img]; ok {
			t.Errorf("Prime path must not merge non-core image %q", img)
		}
	}
	if _, ok := g.LinuxImages["rancher/rancher:v2.13.1"]; !ok {
		t.Errorf("expected allowlisted rancher/rancher from Prime rancher-images.txt")
	}
	if len(g.LinuxImages) != 6 {
		t.Fatalf("expected 6 allowlisted core images, got %d: %v", len(g.LinuxImages), g.LinuxImages)
	}
}
