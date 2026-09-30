package listgenerator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// SUSE Observability charts live in a Prime Helm repo, not rancher/charts.
// Air-gap docs: https://documentation.suse.com/cloudnative/suse-observability/
const (
	SuseObservabilityChartsIndexURL  = "https://charts.rancher.com/server-charts/prime/suse-observability/index.yaml"
	SuseObservabilityChartsBaseURL   = "https://charts.rancher.com/server-charts/prime/suse-observability"
	SuseObservabilityAgentChartName  = "suse-observability-agent"
	SuseObservabilityServerChartName = "suse-observability"
)

// Feature flags matching StackVista o11y-agent-get-images.sh.
const suseObservabilityAgentHelmValues = "" +
	"httpHeaderInjectorWebhook.enabled=true," +
	"stackstate.apiKey=APIKEY," +
	"stackstate.cluster.name=dummy-cluster," +
	"stackstate.url=https://dummy.stackstate.io/stsAgent," +
	"kubernetes-rbac-agent.enabled=true," +
	"otel.enabled=true," +
	"otel.prometheusScraping.enabled=true," +
	"otel.prometheusScraping.monitorCrds.enabled=true," +
	"otel.prometheusScraping.targetAllocator.mtlsEnabled=false"

// Base values matching StackVista o11y-get-images.sh (self-hosted platform).
const suseObservabilityServerHelmValuesBase = "" +
	"stackstate.components.replicationChecker.enabled=true," +
	"ai.assistant.enabled=true," +
	"anomaly-detection.enabled=true," +
	"global.backup.enabled=true," +
	"backup.storage.backend.pvc.enabled=true," +
	"s3proxy.credentials.accessKey=ABCDEFGH," +
	"s3proxy.credentials.secretKey=ABCDEFGHABCDEFGH," +
	"stackstate.baseUrl=http://dummy.stackstate.io," +
	"stackstate.admin.authentication.password=dummy," +
	"stackstate.authentication.adminPassword=dummy," +
	"stackstate.license.key=dummy," +
	"global.receiverApiKey=dummy," +
	"stackstate.k8sAuthorization.enabled=true"

var helmImageLineRE = regexp.MustCompile(`(?i)^\s*image:\s*["']?([^"'#\s]+)["']?`)

// SuseObservabilityAgentSource formats the synthetic chart source for the tree.
func SuseObservabilityAgentSource(version string) string {
	if version == "" {
		version = "latest"
	}
	return fmt.Sprintf("[suse-observability;%s:%s]", SuseObservabilityAgentChartName, version)
}

// SuseObservabilityServerSource formats the synthetic chart source for the
// self-hosted Observability platform chart.
func SuseObservabilityServerSource(version string) string {
	if version == "" {
		version = "latest"
	}
	return fmt.Sprintf("[suse-observability;%s:%s]", SuseObservabilityServerChartName, version)
}

type helmChartIndex struct {
	Entries map[string][]struct {
		Name    string   `yaml:"name"`
		Version string   `yaml:"version"`
		URLs    []string `yaml:"urls"`
	} `yaml:"entries"`
}

// ResolveSuseObservabilityChart returns the latest non-prerelease chart version
// and download URL for the given chart name in the Prime Observability repo.
func ResolveSuseObservabilityChart(ctx context.Context, chartName string) (version, downloadURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, SuseObservabilityChartsIndexURL, nil)
	if err != nil {
		return "", "", err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return "", "", fmt.Errorf("suse-observability index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("suse-observability index: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var idx helmChartIndex
	if err := yaml.Unmarshal(body, &idx); err != nil {
		return "", "", fmt.Errorf("suse-observability index parse: %w", err)
	}
	entries := idx.Entries[chartName]
	if len(entries) == 0 {
		return "", "", fmt.Errorf("suse-observability index: no %s entries", chartName)
	}
	for _, e := range entries {
		v := strings.TrimSpace(e.Version)
		if v == "" || strings.Contains(strings.ToLower(v), "pre") {
			continue
		}
		u := ""
		if len(e.URLs) > 0 {
			u = e.URLs[0]
		}
		if u == "" {
			u = fmt.Sprintf("%s/%s-%s.tgz", SuseObservabilityChartsBaseURL, chartName, v)
		} else if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = strings.TrimRight(SuseObservabilityChartsBaseURL, "/") + "/" + strings.TrimLeft(u, "/")
		}
		return v, u, nil
	}
	return "", "", fmt.Errorf("suse-observability index: no stable %s version", chartName)
}

// ResolveSuseObservabilityAgentChart is a convenience wrapper for the agent chart.
func ResolveSuseObservabilityAgentChart(ctx context.Context) (version, downloadURL string, err error) {
	return ResolveSuseObservabilityChart(ctx, SuseObservabilityAgentChartName)
}

func requireHelm() error {
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("helm not found on PATH (required to template SUSE Observability charts): %w", err)
	}
	return nil
}

func downloadSuseObservabilityChart(ctx context.Context, chartName string) (tgzPath, version, tmpDir string, err error) {
	version, url, err := ResolveSuseObservabilityChart(ctx, chartName)
	if err != nil {
		return "", "", "", err
	}
	tmpDir, err = os.MkdirTemp("", "genesis-suse-o11y-*")
	if err != nil {
		return "", "", "", err
	}
	tgzPath = filepath.Join(tmpDir, fmt.Sprintf("%s-%s.tgz", chartName, version))
	if err := downloadFile(ctx, url, tgzPath); err != nil {
		os.RemoveAll(tmpDir)
		return "", version, "", fmt.Errorf("download %s: %w", url, err)
	}
	return tgzPath, version, tmpDir, nil
}

func helmTemplateImages(ctx context.Context, tgzPath string, setValues ...string) ([]string, error) {
	args := []string{"template", "release", tgzPath}
	for _, v := range setValues {
		if v == "" {
			continue
		}
		args = append(args, "--set", v)
	}
	cmd := exec.CommandContext(ctx, "helm", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("helm template: %w\n%s", err, truncateBytes(out, 2048))
	}
	imgSet := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		m := helmImageLineRE.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		img := strings.TrimSpace(m[1])
		if img == "" || img == "null" || strings.HasPrefix(img, "{{") {
			continue
		}
		imgSet[img] = true
	}
	images := make([]string, 0, len(imgSet))
	for img := range imgSet {
		images = append(images, img)
	}
	sort.Strings(images)
	return images, nil
}

func mergeImageSet(dst map[string]bool, imgs []string) {
	for _, img := range imgs {
		dst[img] = true
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FetchSuseObservabilityAgentImages downloads the agent chart and extracts
// images via `helm template` (same values as o11y-agent-get-images.sh).
func FetchSuseObservabilityAgentImages(ctx context.Context) (images []string, version string, err error) {
	if err := requireHelm(); err != nil {
		return nil, "", err
	}
	tgzPath, version, tmpDir, err := downloadSuseObservabilityChart(ctx, SuseObservabilityAgentChartName)
	if err != nil {
		return nil, version, err
	}
	defer os.RemoveAll(tmpDir)

	imgSet := make(map[string]bool)
	for _, otelLogs := range []bool{false, true} {
		logsCollector := "logsAgent"
		if otelLogs {
			logsCollector = "otelLogsAgent"
		}
		imgs, err := helmTemplateImages(ctx, tgzPath,
			suseObservabilityAgentHelmValues,
			fmt.Sprintf("%s.enabled=true", logsCollector),
			fmt.Sprintf("global.features.experimentalOtelLogsAgent=%v", otelLogs),
		)
		if err != nil {
			return nil, version, fmt.Errorf("agent chart (otelLogs=%v): %w", otelLogs, err)
		}
		mergeImageSet(imgSet, imgs)
	}
	return sortedKeys(imgSet), version, nil
}

// FetchSuseObservabilityServerImages downloads the self-hosted platform chart
// and extracts images via `helm template` (same values as o11y-get-images.sh:
// Distributed + Mono HBase modes).
func FetchSuseObservabilityServerImages(ctx context.Context) (images []string, version string, err error) {
	if err := requireHelm(); err != nil {
		return nil, "", err
	}
	tgzPath, version, tmpDir, err := downloadSuseObservabilityChart(ctx, SuseObservabilityServerChartName)
	if err != nil {
		return nil, version, err
	}
	defer os.RemoveAll(tmpDir)

	imgSet := make(map[string]bool)
	for _, extra := range []string{"", "hbase.deployment.mode=Mono"} {
		imgs, err := helmTemplateImages(ctx, tgzPath, suseObservabilityServerHelmValuesBase, extra)
		if err != nil {
			mode := "Distributed"
			if extra != "" {
				mode = "Mono"
			}
			return nil, version, fmt.Errorf("server chart (%s): %w", mode, err)
		}
		mergeImageSet(imgSet, imgs)
	}
	return sortedKeys(imgSet), version, nil
}

// MergeSuseObservabilityAgentImages fetches agent images and merges them.
func MergeSuseObservabilityAgentImages(ctx context.Context, linuxImages map[string]map[string]bool) (int, string, error) {
	if linuxImages == nil {
		return 0, "", nil
	}
	images, version, err := FetchSuseObservabilityAgentImages(ctx)
	if err != nil {
		return 0, "", err
	}
	source := SuseObservabilityAgentSource(version)
	for _, img := range images {
		utils.AddSourceToImage(linuxImages, img, source)
	}
	logrus.Infof("Included SUSE Observability Agent %s (%d images from %s)",
		version, len(images), SuseObservabilityChartsBaseURL)
	return len(images), version, nil
}

// MergeSuseObservabilityServerImages fetches self-hosted platform images and merges them.
func MergeSuseObservabilityServerImages(ctx context.Context, linuxImages map[string]map[string]bool) (int, string, error) {
	if linuxImages == nil {
		return 0, "", nil
	}
	images, version, err := FetchSuseObservabilityServerImages(ctx)
	if err != nil {
		return 0, "", err
	}
	source := SuseObservabilityServerSource(version)
	for _, img := range images {
		utils.AddSourceToImage(linuxImages, img, source)
	}
	logrus.Infof("Included SUSE Observability Server %s (%d images from %s)",
		version, len(images), SuseObservabilityChartsBaseURL)
	return len(images), version, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
