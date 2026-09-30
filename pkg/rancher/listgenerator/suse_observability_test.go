package listgenerator

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func Test_SuseObservabilityAgentSource(t *testing.T) {
	got := SuseObservabilityAgentSource("1.6.12")
	want := "[suse-observability;suse-observability-agent:1.6.12]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func Test_SuseObservabilityServerSource(t *testing.T) {
	got := SuseObservabilityServerSource("2.11.2")
	want := "[suse-observability;suse-observability:2.11.2]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func Test_FetchSuseObservabilityAgentImages_Live(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	images, version, err := FetchSuseObservabilityAgentImages(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if version == "" {
		t.Fatal("empty version")
	}
	if len(images) < 5 {
		t.Fatalf("expected several agent images, got %d: %v", len(images), images)
	}
	foundAgent := false
	for _, img := range images {
		if !strings.Contains(img, "suse-observability/") && !strings.Contains(img, "stackstate/") {
			t.Errorf("unexpected image registry/path: %s", img)
		}
		if strings.Contains(img, "stackstate-k8s-agent") || strings.Contains(img, "stackstate-k8s-cluster-agent") {
			foundAgent = true
		}
	}
	if !foundAgent {
		t.Fatalf("missing core agent image in %v", images)
	}
	t.Logf("version=%s images=%d", version, len(images))
}

func Test_FetchSuseObservabilityServerImages_Live(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	images, version, err := FetchSuseObservabilityServerImages(ctx)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if version == "" {
		t.Fatal("empty version")
	}
	if len(images) < 20 {
		t.Fatalf("expected ~37 server images, got %d: %v", len(images), images)
	}
	needles := []string{"elasticsearch", "kafka", "hbase"}
	for _, n := range needles {
		found := false
		for _, img := range images {
			if strings.Contains(img, n) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s image in server list (%d images)", n, len(images))
		}
	}
	t.Logf("version=%s images=%d", version, len(images))
}

func Test_helmImageLineRE(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{`        image: "registry.rancher.com/suse-observability/stackstate-k8s-agent:dd1cba03"`, "registry.rancher.com/suse-observability/stackstate-k8s-agent:dd1cba03"},
		{`        image: registry.rancher.com/suse-observability/promtail:3.6.11-so19`, "registry.rancher.com/suse-observability/promtail:3.6.11-so19"},
		{`        image: 'registry.rancher.com/x:y'`, "registry.rancher.com/x:y"},
		{`      name: something`, ""},
	}
	for _, tc := range cases {
		m := helmImageLineRE.FindStringSubmatch(tc.line)
		got := ""
		if len(m) >= 2 {
			got = m[1]
		}
		if got != tc.want {
			t.Errorf("line %q: got %q want %q", tc.line, got, tc.want)
		}
	}
}
