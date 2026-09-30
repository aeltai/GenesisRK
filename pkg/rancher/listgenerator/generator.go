package listgenerator

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/rancher/rke/types/kdm"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type GeneratorOption struct {
	RancherVersion string
	MinKubeVersion string

	ChartsPaths map[string]chartimages.ChartRepoType // map[url]type
	ChartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	KDMPath string // The path of KDM data.json file.
	KDMURL  string // The remote URL of KDM data.json.

	// ImageListBaseURL when set (e.g. Rancher Prime https://prime.ribs.rancher.io) uses that base for
	// K3s/RKE2 per-version image lists (via KDM getters) and for allowlisted core images from
	// {base}/rancher/{version}/rancher-images.txt. It does NOT dump the full rancher-images.txt
	// matrix — same allowlist behavior as RancherImagesTxtURL / Community.
	ImageListBaseURL string

	// RancherImagesTxtURL when set (community mode) fetches the official
	// rancher-images.txt release asset and merges ONLY the allowlisted core
	// images (rancher-agent, shell, machine, system-agent, kubectl) that are
	// not discoverable from charts or KDM. Keeps the generated list small
	// while making it complete for air-gapped downstream provisioning.
	RancherImagesTxtURL string

	InsecureSkipTLS     bool
	RemoveDeprecatedKDM bool

	// IncludeClusterTypes limits which cluster types are included (K3S, RKE2, RKE). Empty = all.
	IncludeClusterTypes []kdmimages.ClusterType
	IncludeK3sVersions  []string
	IncludeRKE2Versions []string
	IncludeRKE1Versions []string

	// LinuxArch selects RKE2 linux image list (linux-amd64.txt or linux-arm64.txt). Default amd64.
	LinuxArch string

	IncludeChartImages bool
	IncludeChartNames  []string

	AppCollectionCharts []string
	AppCollectionImages []string

	// OnProgress reports generation progress: phase key, current step, total steps, detail label.
	OnProgress func(phase string, current, total int, detail string)
}

// Generator is a generator to generate image list from charts, KDM data, etc.
type Generator struct {
	rancherVersion string // Rancher version, should be va.b.c
	minKubeVersion string // Minimum RKE1 kube verision, should be va.b.c

	chartsPaths map[string]chartimages.ChartRepoType // map[url]type
	chartURLs   map[string]struct {
		Type   chartimages.ChartRepoType
		Branch string
	}

	kdmPath             string
	kdmURL              string
	imageListBaseURL    string
	rancherImagesTxtURL string
	linuxArch           string

	insecureSkipTLS     bool
	removeDeprecatedKDM bool

	includeClusterTypes map[kdmimages.ClusterType]bool
	includeK3sVersions  map[string]bool
	includeRKE2Versions map[string]bool
	includeRKE1Versions map[string]bool
	includeChartImages  bool
	includeChartNames   map[string]bool
	appCollectionCharts []string
	appCollectionImages []string
	onProgress          func(phase string, current, total int, detail string)

	chartTotal int
	chartDone  atomic.Int32

	// All generated images, map[image]map[source]true
	LinuxImages   map[string]map[string]bool
	WindowsImages map[string]map[string]bool

	RKE1LinuxImages   map[string]map[string]bool
	RKE2LinuxImages   map[string]map[string]bool
	K3sLinuxImages    map[string]map[string]bool
	RKE2WindowsImages map[string]map[string]bool

	RKE1Versions map[string]bool
	RKE2Versions map[string]bool
	K3sVersions  map[string]bool

	// ChartMetadata is per-chart display metadata (icon URL, description,
	// version, repo) collected from the Helm repo indexes. map[chartName].
	ChartMetadata map[string]chartimages.ChartMetadata
}

func NewGenerator(o *GeneratorOption) (*Generator, error) {
	if o.RancherVersion == "" {
		return nil, fmt.Errorf("invalid rancher version")
	}
	rancherVersion, err := utils.EnsureSemverValid(o.RancherVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher version: %v", o.RancherVersion)
	}
	if o.ChartURLs == nil && o.ChartsPaths == nil &&
		o.KDMPath == "" && o.KDMURL == "" &&
		len(o.AppCollectionCharts) == 0 && len(o.AppCollectionImages) == 0 {
		return nil, fmt.Errorf("no input source provided")
	}

	includeClusterTypes := make(map[kdmimages.ClusterType]bool)
	for _, t := range o.IncludeClusterTypes {
		includeClusterTypes[t] = true
	}
	includeK3sVersions := make(map[string]bool)
	for _, v := range o.IncludeK3sVersions {
		includeK3sVersions[v] = true
	}
	includeRKE2Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE2Versions {
		includeRKE2Versions[v] = true
	}
	includeRKE1Versions := make(map[string]bool)
	for _, v := range o.IncludeRKE1Versions {
		includeRKE1Versions[v] = true
	}
	includeChartNames := make(map[string]bool)
	for _, name := range o.IncludeChartNames {
		includeChartNames[name] = true
	}

	g := &Generator{
		rancherVersion:      rancherVersion,
		minKubeVersion:      o.MinKubeVersion,
		chartsPaths:         o.ChartsPaths,
		chartURLs:           o.ChartURLs,
		kdmPath:             o.KDMPath,
		kdmURL:              o.KDMURL,
		imageListBaseURL:    o.ImageListBaseURL,
		rancherImagesTxtURL: o.RancherImagesTxtURL,
		linuxArch:           kdmimages.NormalizeLinuxArch(o.LinuxArch),

		insecureSkipTLS:     o.InsecureSkipTLS,
		removeDeprecatedKDM: o.RemoveDeprecatedKDM,

		includeClusterTypes: includeClusterTypes,
		includeK3sVersions:  includeK3sVersions,
		includeRKE2Versions: includeRKE2Versions,
		includeRKE1Versions: includeRKE1Versions,
		includeChartImages:  o.IncludeChartImages,
		includeChartNames:   includeChartNames,
		appCollectionCharts: o.AppCollectionCharts,
		appCollectionImages: o.AppCollectionImages,
		onProgress:          o.OnProgress,

		LinuxImages:       make(map[string]map[string]bool),
		WindowsImages:     make(map[string]map[string]bool),
		K3sLinuxImages:    make(map[string]map[string]bool),
		K3sVersions:       make(map[string]bool),
		RKE1LinuxImages:   make(map[string]map[string]bool),
		RKE1Versions:      make(map[string]bool),
		RKE2LinuxImages:   make(map[string]map[string]bool),
		RKE2WindowsImages: make(map[string]map[string]bool),
		RKE2Versions:      make(map[string]bool),

		ChartMetadata: make(map[string]chartimages.ChartMetadata),
	}
	return g, nil
}

// mergeChartMetadata merges chart metadata collected by a Chart fetch into
// the generator; the first repo to provide a chart's metadata wins.
func (g *Generator) mergeChartMetadata(meta map[string]chartimages.ChartMetadata) {
	for name, m := range meta {
		if _, ok := g.ChartMetadata[name]; !ok {
			g.ChartMetadata[name] = m
		}
	}
}

type chartFetchResult struct {
	linuxImages   map[string]map[string]bool
	windowsImages map[string]map[string]bool
	metadata      map[string]chartimages.ChartMetadata
}

func fetchChartRepoOS(ctx context.Context, c chartimages.Chart) (map[string]map[string]bool, map[string]chartimages.ChartMetadata, error) {
	if err := c.FetchImages(ctx); err != nil {
		return nil, nil, err
	}
	out := make(map[string]map[string]bool, len(c.ImageSet))
	for image, sources := range c.ImageSet {
		if chartimages.IgnoreChartImages[image] {
			continue
		}
		out[image] = sources
	}
	return out, c.Metadata, nil
}

func fetchChartRepo(ctx context.Context, c chartimages.Chart) (*chartFetchResult, error) {
	linuxChart := c
	linuxChart.OS = chartimages.Linux
	linuxChart.ImageSet = nil
	linuxImages, meta, err := fetchChartRepoOS(ctx, linuxChart)
	if err != nil {
		return nil, err
	}

	windowsChart := c
	windowsChart.OS = chartimages.Windows
	windowsChart.ImageSet = nil
	windowsImages, winMeta, err := fetchChartRepoOS(ctx, windowsChart)
	if err != nil {
		return nil, err
	}
	for name, m := range winMeta {
		if _, ok := meta[name]; !ok {
			meta[name] = m
		}
	}

	return &chartFetchResult{
		linuxImages:   linuxImages,
		windowsImages: windowsImages,
		metadata:      meta,
	}, nil
}

func (g *Generator) mergeChartFetchResult(r *chartFetchResult) {
	for image, sources := range r.linuxImages {
		for source := range sources {
			utils.AddSourceToImage(g.LinuxImages, image, source)
		}
	}
	for image, sources := range r.windowsImages {
		for source := range sources {
			utils.AddSourceToImage(g.WindowsImages, image, source)
		}
	}
	g.mergeChartMetadata(r.metadata)
}

func (g *Generator) reportProgress(phase string, current, total int, detail string) {
	if g.onProgress != nil {
		g.onProgress(phase, current, total, detail)
	}
}

func (g *Generator) Run(ctx context.Context) error {
	g.chartTotal = len(g.chartsPaths) + len(g.chartURLs)
	if g.chartTotal > 0 {
		g.reportProgress("charts", 0, g.chartTotal, "")
	}
	if err := g.generateFromChartPaths(ctx); err != nil {
		return err
	}
	if err := g.generateFromChartURLs(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMPath(ctx); err != nil {
		return err
	}
	if err := g.generateFromKDMURL(ctx); err != nil {
		return err
	}
	if err := g.generateFromPrimeRancherImages(ctx); err != nil {
		return err
	}
	if err := g.generateCoreImagesFromRancherImagesTxt(ctx); err != nil {
		// Non-fatal: RC/alpha releases may not have the asset published yet.
		logrus.Warnf("Could not merge core Rancher images from rancher-images.txt: %v", err)
	}
	if err := g.generateFromAppCollection(ctx); err != nil {
		return err
	}
	return nil
}

// RancherCoreImagesSource is the source tag applied to core Rancher images
// merged from the official rancher-images.txt release asset.
const RancherCoreImagesSource = "[rancher-core]"

// rancherCoreImageRepos is the allowlist of image repositories extracted from
// the official rancher-images.txt. These images are referenced by Rancher's
// own settings (not by any chart or KDM data), so the chart/KDM based
// generator cannot discover them, yet an air-gapped Rancher cannot provision
// or import downstream clusters without them.
var rancherCoreImageRepos = map[string]bool{
	"rancher/rancher":       true, // main server (pins the exact tag)
	"rancher/rancher-agent": true, // downstream/imported cluster agent
	"rancher/shell":         true, // kubectl shell used by the Rancher UI
	"rancher/machine":       true, // node driver provisioning
	"rancher/system-agent":  true, // v2prov system agent (SUC variant)
	"rancher/kubectl":       true, // helm-operation jobs
}

const primeImageListBaseURL = "https://prime.ribs.rancher.io"

// generateCoreImagesFromRancherImagesTxt fetches the official rancher-images.txt
// (GitHub release asset in community mode) and merges ONLY the allowlisted core
// images into LinuxImages, with versions pinned to the chosen Rancher release.
// On GitHub 404, falls back to prime.ribs.rancher.io for the same allowlist.
func (g *Generator) generateCoreImagesFromRancherImagesTxt(ctx context.Context) error {
	if g.rancherImagesTxtURL == "" {
		return nil
	}
	g.reportProgress("core", 0, 1, "")
	logrus.Infof("Get core Rancher images from %q", g.rancherImagesTxtURL)
	merged, status, err := g.fetchAndMergeCoreRancherImages(ctx, g.rancherImagesTxtURL)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		version := strings.TrimPrefix(g.rancherVersion, "v")
		fallback := fmt.Sprintf("%s/rancher/v%s/rancher-images.txt",
			strings.TrimSuffix(primeImageListBaseURL, "/"), version)
		if fallback != g.rancherImagesTxtURL {
			logrus.Infof("rancher-images.txt not on GitHub release; trying Prime registry %q", fallback)
			var fbStatus int
			merged, fbStatus, err = g.fetchAndMergeCoreRancherImages(ctx, fallback)
			if err != nil {
				return err
			}
			if fbStatus != http.StatusOK {
				return fmt.Errorf("rancher-images.txt: %s returned %d", g.rancherImagesTxtURL, status)
			}
		}
	} else if status != http.StatusOK {
		return fmt.Errorf("rancher-images.txt: %s returned %d", g.rancherImagesTxtURL, status)
	}
	logrus.Infof("Merged %d core Rancher images from rancher-images.txt", merged)
	g.reportProgress("core", 1, 1, "")
	return nil
}

func (g *Generator) fetchAndMergeCoreRancherImages(ctx context.Context, imageURL string) (merged int, status int, err error) {
	client := &http.Client{
		Timeout: 90 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: g.insecureSkipTLS},
			Proxy:           http.ProxyFromEnvironment,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("rancher-images.txt: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return 0, 0, fmt.Errorf("rancher-images.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, resp.StatusCode, nil
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "docker.io/")
		repo := line
		if i := strings.LastIndex(repo, ":"); i > 0 {
			repo = repo[:i]
		}
		if !rancherCoreImageRepos[repo] {
			continue
		}
		if g.LinuxImages[line] == nil {
			g.LinuxImages[line] = make(map[string]bool)
		}
		g.LinuxImages[line][RancherCoreImagesSource] = true
		merged++
	}
	if err := sc.Err(); err != nil {
		return merged, http.StatusOK, err
	}
	return merged, http.StatusOK, nil
}

func (g *Generator) noteChartStart(detail string) {
	if g.chartTotal == 0 {
		return
	}
	done := int(g.chartDone.Load())
	g.reportProgress("charts", done, g.chartTotal, detail)
}

func (g *Generator) noteChartDone(detail string) {
	if g.chartTotal == 0 {
		return
	}
	done := int(g.chartDone.Add(1))
	g.reportProgress("charts", done, g.chartTotal, detail)
}

func (g *Generator) generateFromChartPaths(ctx context.Context) error {
	if len(g.chartsPaths) == 0 {
		return nil
	}
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	for path, repoType := range g.chartsPaths {
		path, repoType := path, repoType
		eg.Go(func() error {
			g.noteChartStart(path)
			result, err := fetchChartRepo(ctx, chartimages.Chart{
				RancherVersion: g.rancherVersion,
				Type:           repoType,
				Path:           path,
			})
			if err != nil {
				return err
			}
			mu.Lock()
			g.mergeChartFetchResult(result)
			mu.Unlock()
			g.noteChartDone(path)
			return nil
		})
	}
	return eg.Wait()
}

func (g *Generator) generateFromChartURLs(ctx context.Context) error {
	if len(g.chartURLs) == 0 {
		return nil
	}
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	for url, cfg := range g.chartURLs {
		url, cfg := url, cfg
		eg.Go(func() error {
			g.noteChartStart(url)
			result, err := fetchChartRepo(ctx, chartimages.Chart{
				RancherVersion:  g.rancherVersion,
				Type:            cfg.Type,
				Branch:          cfg.Branch,
				URL:             url,
				InsecureSkipTLS: g.insecureSkipTLS,
			})
			if err != nil {
				return err
			}
			mu.Lock()
			g.mergeChartFetchResult(result)
			mu.Unlock()
			g.noteChartDone(url)
			return nil
		})
	}
	return eg.Wait()
}

func (g *Generator) generateFromKDMPath(ctx context.Context) error {
	if g.kdmPath == "" {
		return nil
	}
	g.reportProgress("kdm", 0, 1, g.kdmPath)
	b, err := os.ReadFile(g.kdmPath)
	if err != nil {
		return err
	}
	g.reportProgress("kdm", 1, 1, "")
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMURL(ctx context.Context) error {
	if g.kdmURL == "" {
		return nil
	}
	g.reportProgress("kdm", 0, 1, g.kdmURL)
	logrus.Infof("Get KDM data from URL: %q", g.kdmURL)

	client := &http.Client{
		Timeout: time.Second * 15,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: g.insecureSkipTLS,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.kdmURL, nil)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("generateFromKDMURL: %w", err)
	}
	g.reportProgress("kdm", 1, 1, "")
	return g.generateFromKDMData(ctx, b)
}

func (g *Generator) generateFromKDMData(ctx context.Context, b []byte) error {
	data, err := kdm.FromData(b)
	if err != nil {
		return fmt.Errorf("generateFromKDMData: %w", err)
	}
	clusters := []kdmimages.ClusterType{
		kdmimages.K3S,
		kdmimages.RKE2,
	}
	if ok, _ := utils.SemverCompare(g.rancherVersion, "v2.12.0-0"); ok < 0 {
		clusters = append(clusters, kdmimages.RKE)
	}
	// When config/TUI limited distros, only fetch those cluster types
	if len(g.includeClusterTypes) > 0 {
		filtered := clusters[:0]
		for _, t := range clusters {
			if g.includeClusterTypes[t] {
				filtered = append(filtered, t)
			}
		}
		clusters = filtered
	}
	total := len(clusters)
	if total > 0 {
		g.reportProgress("distros", 0, total, "")
	}
	var mu sync.Mutex
	var done atomic.Int32
	eg, ctx := errgroup.WithContext(ctx)
	for _, t := range clusters {
		t := t
		eg.Go(func() error {
			opts := &kdmimages.GetterOptions{
				Type:             t,
				RancherVersion:   g.rancherVersion,
				MinKubeVersion:   g.minKubeVersion,
				KDMData:          data,
				ImageListBaseURL: g.imageListBaseURL,
				LinuxArch:        g.linuxArch,
				InsecureSkipTLS:  g.insecureSkipTLS,
				RemoveDeprecated: g.removeDeprecatedKDM,
			}
			switch t {
			case kdmimages.K3S:
				for v := range g.includeK3sVersions {
					opts.IncludeVersions = append(opts.IncludeVersions, v)
				}
			case kdmimages.RKE2:
				for v := range g.includeRKE2Versions {
					opts.IncludeVersions = append(opts.IncludeVersions, v)
				}
			case kdmimages.RKE:
				for v := range g.includeRKE1Versions {
					opts.IncludeVersions = append(opts.IncludeVersions, v)
				}
			}
			getter, err := kdmimages.NewGetter(opts)
			if err != nil {
				return err
			}
			if err = getter.Get(ctx); err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()
			utils.MergeImageSourceSet(g.LinuxImages, getter.LinuxImageSet())
			utils.MergeImageSourceSet(g.WindowsImages, getter.WindowsImageSet())
			switch getter.Source() {
			case kdmimages.RKE:
				utils.MergeSets(g.RKE1Versions, getter.VersionSet())
				utils.MergeImageSourceSet(g.RKE1LinuxImages, getter.LinuxImageSet())
			case kdmimages.RKE2:
				utils.MergeSets(g.RKE2Versions, getter.VersionSet())
				utils.MergeImageSourceSet(g.RKE2LinuxImages, getter.LinuxImageSet())
				utils.MergeImageSourceSet(g.RKE2WindowsImages, getter.WindowsImageSet())
			case kdmimages.K3S:
				utils.MergeSets(g.K3sVersions, getter.VersionSet())
				utils.MergeImageSourceSet(g.K3sLinuxImages, getter.LinuxImageSet())
			}
			d := int(done.Add(1))
			g.reportProgress("distros", d, total, string(t))
			return nil
		})
	}
	return eg.Wait()
}

// generateFromPrimeRancherImages fetches rancher-images.txt from Prime base URL when set
// and merges ONLY the allowlisted core images — same policy as Community
// (generateCoreImagesFromRancherImagesTxt). Distro images still come from KDM +
// per-version lists under ImageListBaseURL (prime.ribs), filtered by selected
// K3s/RKE2 versions. Dumping the full Prime matrix was causing Essentials to
// include every historical hardened-coredns/flannel tag for the Rancher release.
func (g *Generator) generateFromPrimeRancherImages(ctx context.Context) error {
	if g.imageListBaseURL == "" {
		return nil
	}
	g.reportProgress("prime", 0, 1, "")
	version := strings.TrimPrefix(g.rancherVersion, "v")
	url := fmt.Sprintf("%s/rancher/v%s/rancher-images.txt", strings.TrimSuffix(g.imageListBaseURL, "/"), version)
	logrus.Infof("Get core Rancher Prime images from %q (allowlist only)", url)
	merged, status, err := g.fetchAndMergeCoreRancherImages(ctx, url)
	if err != nil {
		return fmt.Errorf("prime rancher-images: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("prime rancher-images: %s returned %d", url, status)
	}
	logrus.Infof("Merged %d core Rancher images from Prime rancher-images.txt", merged)
	g.reportProgress("prime", 1, 1, "")
	return nil
}

func (g *Generator) generateFromAppCollection(ctx context.Context) error {
	if len(g.appCollectionCharts) == 0 && len(g.appCollectionImages) == 0 {
		return nil
	}
	total := len(g.appCollectionImages)
	if total == 0 {
		total = 1
	}
	g.reportProgress("appcollection", 0, total, "")
	const source = "[app-collection]"
	done := 0
	for _, imageRef := range g.appCollectionImages {
		if imageRef == "" {
			continue
		}
		if g.LinuxImages[imageRef] == nil {
			g.LinuxImages[imageRef] = make(map[string]bool)
		}
		g.LinuxImages[imageRef][source] = true
		done++
		g.reportProgress("appcollection", done, total, imageRef)
	}
	// OCI chart refs (oci://dp.apps.rancher.io/charts/...) require helm pull;
	// chartimages currently supports only path and git URL. Skip chart image extraction for now.
	if len(g.appCollectionCharts) > 0 {
		logrus.Debugf("App Collection chart refs (%d) not yet supported for image extraction", len(g.appCollectionCharts))
	}
	if done == 0 {
		g.reportProgress("appcollection", 1, total, "")
	}
	return nil
}
