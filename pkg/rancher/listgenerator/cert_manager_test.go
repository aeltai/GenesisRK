package listgenerator

import (
	"strings"
	"testing"
)

func TestCertManagerImages(t *testing.T) {
	imgs := CertManagerImages(DefaultCertManagerVersion)
	if len(imgs) != 5 {
		t.Fatalf("got %d images, want 5", len(imgs))
	}
	for _, img := range imgs {
		if !strings.HasPrefix(img, "quay.io/jetstack/") {
			t.Errorf("unexpected registry: %s", img)
		}
		if !strings.HasSuffix(img, ":"+DefaultCertManagerVersion) {
			t.Errorf("unexpected tag: %s", img)
		}
	}
}

func TestMergeCertManagerImages(t *testing.T) {
	linux := make(map[string]map[string]bool)
	MergeCertManagerImages(linux, DefaultCertManagerVersion)
	if len(linux) != 5 {
		t.Fatalf("merged %d images, want 5", len(linux))
	}
	wantSource := CertManagerSource(DefaultCertManagerVersion)
	for img, sources := range linux {
		if !sources[wantSource] {
			t.Errorf("%s missing source %q (got %v)", img, wantSource, sources)
		}
	}
}
