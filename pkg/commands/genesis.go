package commands

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cnrancher/hangar/pkg/hangar"
	"github.com/cnrancher/hangar/pkg/image/scan"
	"github.com/cnrancher/hangar/pkg/rancher/appcollection"
	"github.com/cnrancher/hangar/pkg/rancher/chartimages"
	"github.com/cnrancher/hangar/pkg/rancher/kdmimages"
	"github.com/cnrancher/hangar/pkg/rancher/listgenerator"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/rancher/rke/types/kdm"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// drawHero displays the Geeko chameleon ASCII art full-width, then "HANGAR GENESIS" on one line below.
func drawHero() {
	cyan := "\033[36m"
	reset := "\033[0m"
	gecko := []string{
		"                                                         *-     *#*=                                ",
		"                                                        +###.  =###########*:                      ",
		"                                                          +###+ #################*-                ",
		"                                                      ####+  :##*#####################             ",
		"                                                       *################################+          ",
		"                                                                  =#######################+        ",
		"                                   .....:..::::::::.:.....:..-+:  -####################***##       ",
		"                              ..:::::::::::::::::::::::..:::.:###################+:::::......      ",
		"                           .:::::.:::::::::::::::::::::::::::::::##############*:::..          .     ",
		"                        ..:::::.:::::::::::::::::::::::::::::::##############*:::..          .     ",
		"                     ..::::...:::.:..::::::::::::::.::::.::.:::-############*:.....           .    ",
		"                   ..::::...:::.::::...:::.::::.:::::::.::::::::=###########-:...:       ##*  ..   ",
		"                  .:::::.:.::::::::.:::::...:::::::::::.:::::::::+##########:::....          ...   ",
		"                ..::::::.::.::::..:.::::::.:::::::::::::::::::::::+#########+=:::...        ..:--  ",
		"               ..:::::..             .....:::.......:::..:.::::::::=########+++++=:::::::-=++++++  ",
		"              ...::..                   --:::::::..    .:::::::::::::#######*+++++++++++++++++++:  ",
		"              ..:::.                      ###**=:    ..:::::::::::::::+######++++++++++++++++++    ",
		"             ...:..       ........:..  .#- ::::-###*#####=-:::::::::::::######+++++++++++++++-     ",
		"             .....       :::::..-++*#*  ###****+#+  ##########=::::.=########**-==++++++=-         ",
		"           .-+***#####################  ##=-:::::-=+*#########-:::::####+-                         ",
		"  ####################+=+=-.         .:.*#     +****+#+        .:::::.-**##=                       ",
		"=**=-.       .:..::      ....        ..::      -#####+#           .::::-#####                      ",
		"              ......                 ...:       :######++*-         ..:::::-+::+#*                 ",
		"               .:::.:               .:...        +#########:          .:::...:.                    ",
		"                ..:.....          ......                                                           ",
		"                  .:::......:::........                                                            ",
		"                    .::..:...........                                                              ",
		"                        .:.:::::..                                                                ",
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, line := range gecko {
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(cyan)
	b.WriteString(reset)
	b.WriteString("\n")
	fmt.Print(b.String())
}

type genesisOpts struct {
	registry            string
	kdm                 string
	output              string
	outputWindows       string
	outputSource        string
	outputVersions      string
	rancherVersion      string   // single version or display string like "v2.13.1 + v2.14.0-alpha2"
	rancherVersionsList []string // when set (multi-version from API), one rancher/rancher image per entry
	minKubeVersion      string
	dev                 bool
	tlsVerify           bool
	charts              []string
	systemCharts        []string
	autoYes             bool

	rke1Images          string
	rke2Images          string
	rke2WindowsImages   string
	k3sImages           string
	kdmRemoveDeprecated bool

	// Interactive and component selection flags
	interactive     bool
	tui             bool
	components      string
	k3sVersions     string
	rke2Versions    string
	rkeVersions     string
	chartsSelection string

	// Interactive post-run selection (Step 2 & 3); set after generator run
	interactiveSelectedComponentIDs []string
	interactiveSelectedChartNames   []string
	// Exact image refs from TUI tree (so output matches preview)
	interactiveSelectedImageRefs []string

	// Step 1: selected CNI for Standard preset (cni_canal, cni_calico, cni_flannel, cni, or "")
	interactiveSelectedCNI string
	// Step 1: load balancer choices per distro (when any is false, those LB images are excluded from Basic)
	interactiveIncludeLB     bool // legacy: when false, all LB excluded
	interactiveLBK3sKlipper  bool // K3s: Klipper service LB
	interactiveLBK3sTraefik  bool // K3s: Traefik ingress
	interactiveLBRKE2Nginx   bool // RKE2: NGINX Ingress
	interactiveLBRKE2Traefik bool // RKE2: Traefik ingress
	// Step 1: include Windows node images (RKE2/K3s); when false, only Linux images are included
	interactiveIncludeWindows bool
	// Target linux node architecture for RKE2 image lists (amd64, arm64).
	targetArch string

	// Scan: run hangar scan on the final image list and add results to output
	scan        bool
	scanJobs    int
	scanTimeout time.Duration
	scanReport  string

	// Config file for non-interactive mode
	configFile string
	// Save current TUI selections to this YAML config path (after run)
	saveConfigFile string
}

type genesisCmd struct {
	*baseCmd
	*genesisOpts

	isRPMGC                    bool
	includeCommunityImageLists bool // GitHub k3s/rke2/rancher-images.txt (default true)
	includeAppCollectionCharts bool                        // include charts from dp.apps.rancher.io (Application Collection)
	includePartnerCharts       bool                        // include charts from rancher/partner-charts
	includeUIPluginCharts      bool                        // include charts from rancher/ui-plugin-charts
	includeCertManager         bool                        // include jetstack cert-manager (required for Rancher Helm install; default true)
	appCollectionAPIUser       string                      // username for api.apps.rancher.io (set when user is prompted)
	appCollectionAPIPassword   string                      // password/token for api.apps.rancher.io
	appCollectionChartRefs     []string                    // OCI chart refs (oci://dp.apps.rancher.io/charts/<slug>) for tree display
	appCollectionApps          []appcollection.Application // full metadata (name, description, logo, category) for tree display
	keepChartCache             bool                        // serve mode: keep chart clone cache between generate runs
	generator                  *listgenerator.Generator
}

func newGenesisCmd() *genesisCmd {
	cc := &genesisCmd{
		genesisOpts:                new(genesisOpts),
		includeCommunityImageLists: true,
		includeCertManager:         true,
	}

	cc.baseCmd = newBaseCmd(&cobra.Command{
		Use:     "genesis",
		Aliases: []string{"generate-list-genesis"},
		Short:   "Hangar Genesis 0.1 - Generate Rancher Charts & KDM image list for air-gapped scenarios using this hangar extension",
		Long: `'genesis' generates an image list and k8s version list from KDM data and Chart repos of Rancher.
Designed for air-gapped deployment scenarios, this tool helps create comprehensive image manifests
for offline Kubernetes environments.

Genesis supports two modes:

1. Interactive mode (recommended):
    hangar genesis --rancher="v2.13.1" --tui
    hangar genesis --rancher="v2.13.1" --interactive

2. YAML config mode (for automation):
    hangar genesis --rancher="v2.13.1" --config=config.yaml

You can also download the KDM JSON file and clone chart repos manually:

    hangar genesis --rancher="v2.13.1" --tui \
        --chart="./chart-repo-dir" \
        --system-chart="./system-chart-repo-dir" \
        --kdm="./kdm-data.json"

See generate-list-config.example.yaml for config file format.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			utils.SetupLogrus(cc.hideLogTime)
			// When stdout is a terminal, move to top and clear so the hero is at the top
			if term.IsTerminal(int(os.Stdout.Fd())) {
				fmt.Print("\033[2J\033[H") // clear screen, cursor to top-left
			}
			fmt.Println() // Add space before hero
			drawHero()
			fmt.Print("\033[33m") // Yellow color
			fmt.Println("Hangar Genesis – Generate image lists for Rancher air-gapped deployments.")
			fmt.Println("  • Charts & KDM: Community (GitHub, releases.rancher.com) or Rancher Prime (Rancher Prime Registry: charts.rancher.com, releases.rancher.com).")
			fmt.Println("  • Distros: K3s, RKE2, RKE1 – select versions, CNI, load balancer, Linux/Windows.")
			fmt.Println("  • Application Collection: optional live-fetch from api.apps.rancher.io (charts + containers).")
			fmt.Println("  • Output: combined image list, per-distro lists, versions file; optional scan.")
			fmt.Println("  Coming: generic Helm and OCI chart integrations into image-list.")
			fmt.Println()
			fmt.Println("Author: ala.eltai@suse.com | GenesisRK https://github.com/aeltai/GenesisRK | Based on https://github.com/cnrancher/hangar (StarryWang)")
			fmt.Print("\033[0m") // Reset color
			fmt.Println()
			fmt.Println("\033[2mTo exit at any time: press Control+C (Mac: the Control key, not Command). Or press q in prompts.\033[0m")
			fmt.Println()
			if cc.debug {
				logrus.SetLevel(logrus.DebugLevel)
				logrus.Debugf("Debug output enabled")
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --tui is set, also set interactive flag
			if cc.tui {
				cc.interactive = true
			}

			// Genesis only supports interactive mode or YAML config mode
			if !cc.interactive && cc.configFile == "" {
				return fmt.Errorf("genesis requires either --interactive/--tui flag or --config file\n\n" +
					"Interactive mode:\n" +
					"  hangar genesis --rancher=v2.13.1 --tui\n" +
					"  hangar genesis --rancher=v2.13.1 --interactive\n\n" +
					"YAML config mode:\n" +
					"  hangar genesis --rancher=v2.13.1 --config=config.yaml\n\n" +
					"See generate-list-config.example.yaml for config file format")
			}

			if err := cc.setupFlags(); err != nil {
				return err
			}
			if err := cc.handleComponentSelection(); err != nil {
				return err
			}
			if err := cc.prepareGenerator(); err != nil {
				return err
			}
			if err := cc.run(signalContext); err != nil {
				return err
			}
			// Skip interactive post-run if config file is provided
			if cc.interactive && cc.configFile == "" {
				if err := cc.interactivePostRunPrompt(); err != nil {
					return err
				}
			} else if cc.configFile != "" {
				// Apply group/chart selections from config
				if err := cc.applyConfigSelections(); err != nil {
					return err
				}
			}
			if err := cc.finish(); err != nil {
				return err
			}
			if cc.saveConfigFile != "" {
				if err := cc.writeSaveConfig(); err != nil {
					return err
				}
			}
			return nil
		},
	})
	flags := cc.baseCmd.cmd.PersistentFlags()
	flags.StringVarP(&cc.registry, "registry", "", "", "customize the registry URL of the generated image list")
	flags.StringVarP(&cc.output, "output", "o", "", "output linux image list file (default \"[RANCHER_VERSION]-images.txt\")")
	flags.StringVarP(&cc.outputWindows, "output-windows", "", "", "output the windows image list if specified")
	flags.StringVarP(&cc.outputSource, "output-source", "", "", "output the image list with image source if specified")
	flags.StringVarP(&cc.outputVersions, "output-versions", "", "", "output Rancher supported k8s versions (default \"[RANCHER_VERSION]-k8s-versions.txt\")")
	flags.StringVarP(&cc.rancherVersion, "rancher", "", "", "rancher version (semver with 'v' prefix) "+
		"(use '-ent' suffix to distinguish with Rancher Prime Manager GC) (required)")
	flags.StringVarP(&cc.minKubeVersion, "min-kube-version", "", "", "min RKE1 kube version when generate images, example: 'v1.28' (optional)")
	flags.BoolVarP(&cc.dev, "dev", "", false, "switch to dev branch/URL of charts & KDM data")
	flags.StringVarP(&cc.kdm, "kdm", "", "", "KDM file path or URL")
	flags.StringSliceVarP(&cc.charts, "chart", "", nil, "cloned chart repo path (URL not supported)")
	flags.StringSliceVarP(&cc.systemCharts, "system-chart", "", nil, "cloned system chart repo path (URL not supported)")
	flags.BoolVarP(&cc.kdmRemoveDeprecated, "kdm-remove-deprecated", "", true, "remove deprecated k3s/rke2 k8s versions from KDM")
	flags.StringVarP(&cc.rke1Images, "rke-images", "", "", "output KDM RKE linux image list if specified")
	flags.StringVarP(&cc.rke2Images, "rke2-images", "", "", "output KDM RKE2 linux image list if specified")
	flags.StringVarP(&cc.rke2WindowsImages, "rke2-windows-images", "", "", "output KDM RKE2 Windows image list if specified")
	flags.StringVarP(&cc.k3sImages, "k3s-images", "", "", "output KDM K3s linux image list if specified")
	flags.BoolVarP(&cc.tlsVerify, "tls-verify", "", true, "require HTTPS and verify certificates")
	flags.BoolVarP(&cc.autoYes, "auto-yes", "y", false, "answer yes automatically (used in shell script)")
	flags.BoolVarP(&cc.interactive, "interactive", "i", false, "interactively select components to include in the image list (required for genesis)")
	flags.BoolVarP(&cc.tui, "tui", "", false, "use terminal UI (arrow keys, Space toggle, ←/→ or Enter on Charts to expand/collapse) (required for genesis)")
	// Note: components, versions, and charts flags are kept for backward compatibility with config file parsing
	// but they are not used in non-interactive mode - only --config or --interactive/--tui are supported
	flags.StringVarP(&cc.components, "components", "", "", "[deprecated for genesis] use --config or --interactive instead")
	flags.StringVarP(&cc.k3sVersions, "k3s-versions", "", "", "[deprecated for genesis] use --config or --interactive instead")
	flags.StringVarP(&cc.rke2Versions, "rke2-versions", "", "", "[deprecated for genesis] use --config or --interactive instead")
	flags.StringVarP(&cc.rkeVersions, "rke-versions", "", "", "[deprecated for genesis] use --config or --interactive instead")
	flags.StringVarP(&cc.chartsSelection, "charts", "", "", "[deprecated for genesis] use --config or --interactive instead")
	flags.BoolVarP(&cc.scan, "scan", "", false, "run vulnerability scan on each image and add scan summary to the output file")
	flags.IntVarP(&cc.scanJobs, "scan-jobs", "", 1, "worker number when --scan (1-20)")
	flags.DurationVarP(&cc.scanTimeout, "scan-timeout", "", 10*time.Minute, "timeout per image when --scan")
	flags.StringVarP(&cc.scanReport, "scan-report", "", "", "scan report file when --scan (default: output base + \"-scan-report.csv\")")
	flags.StringVarP(&cc.configFile, "config", "c", "", "YAML config file for non-interactive mode (overrides interactive mode)")
	flags.StringVarP(&cc.saveConfigFile, "save-config", "", "", "after TUI run, write current selections to this YAML config file (distros, cni, loadBalancer, versions, groups, charts)")

	newGenesisServeCmd(cc)
	return cc
}

func (cc *genesisCmd) setupFlags() error {
	if cc.rancherVersion == "" {
		return fmt.Errorf("rancher version not specified, use '--rancher' to specify the rancher version")
	}
	if !strings.HasPrefix(cc.rancherVersion, "v") {
		cc.rancherVersion = "v" + cc.rancherVersion
	}
	if cc.output == "" {
		cc.output = cc.rancherVersion + "-images.txt"
	}
	if cc.outputVersions == "" {
		cc.outputVersions = cc.rancherVersion + "-versions.txt"
	}
	if strings.Contains(cc.rancherVersion, "-ent") {
		logrus.Infof("Set to Rancher Prime Manager GC version")
		cc.isRPMGC = true
		v := strings.Split(cc.rancherVersion, "-ent")
		cc.rancherVersion = v[0]
	}
	if !semver.IsValid(cc.rancherVersion) {
		return fmt.Errorf("%q is not a valid semver version", cc.rancherVersion)
	}

	return nil
}

func (cc *genesisCmd) handleComponentSelection() error {
	// If --tui is set, also set interactive flag
	if cc.tui {
		cc.interactive = true
	}

	// Genesis only supports interactive mode or config file mode
	// This is enforced in RunE, but double-check here
	if !cc.interactive && cc.configFile == "" {
		return fmt.Errorf("genesis requires either --interactive/--tui or --config flag")
	}

	// If config file is provided, use it instead of interactive mode
	if cc.configFile != "" {
		return cc.loadConfigFile()
	}

	if cc.interactive {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			logrus.Warnf("Interactive mode requires a TTY. Falling back to non-interactive mode.")
			cc.interactive = false
		} else if cc.tui {
			return cc.runStep1TUI()
		} else {
			return cc.interactivePrompt()
		}
	}

	return cc.parseComponentFlags()
}

// generateListConfig represents the YAML config file structure
type generateListConfig struct {
	Distros                    []string            `yaml:"distros"`                    // ["k3s", "rke2", "rke"]
	CNI                        string              `yaml:"cni"`                        // "cni_canal", "cni_calico", "cni_flannel"
	LoadBalancer               *bool               `yaml:"loadBalancer"`               // true = include LB/ingress (K3s: Klipper/Traefik, RKE2: NGINX/Traefik), false = exclude
	IncludeWindows             *bool               `yaml:"includeWindows"`             // true = include Windows node images (RKE2/K3s), false = Linux only (default)
	Arch                       string              `yaml:"arch"`                       // linux node arch for RKE2 lists: amd64 (default) or arm64
	Versions                   map[string][]string `yaml:"versions"`                   // {"k3s": ["v1.28.5"], "rke2": ["v1.28.5"]}
	Groups                     []string            `yaml:"groups"`                     // ["basic", "addons"] or specific chart names
	Charts                     []string            `yaml:"charts"`                     // Specific chart names to include
	SourceType                 string              `yaml:"sourceType"`                 // "community" (default) or "prime-gc" (Rancher Prime Manager GC charts/KDM)
	IncludeAppCollectionCharts *bool               `yaml:"includeAppCollectionCharts"` // true = also include charts from dp.apps.rancher.io (requires helm registry login)
	IncludePartnerCharts       *bool               `yaml:"includePartnerCharts"`       // true = also include rancher/partner-charts
	IncludeUIPluginCharts      *bool               `yaml:"includeUIPluginCharts"`      // true = also include rancher/ui-plugin-charts
	IncludeCertManager         *bool               `yaml:"includeCertManager"`         // true = include jetstack cert-manager images (default true)
	Scan                       *scanConfig         `yaml:"scan"`                       // Optional scan configuration
}

type scanConfig struct {
	Enabled bool          `yaml:"enabled"`
	Jobs    int           `yaml:"jobs"`
	Timeout time.Duration `yaml:"timeout"`
	Report  string        `yaml:"report"`
}

// loadConfigFile loads and parses the YAML config file
func (cc *genesisCmd) loadConfigFile() error {
	data, err := os.ReadFile(cc.configFile)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var config generateListConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	// Set distros (components)
	if len(config.Distros) > 0 {
		cc.components = strings.Join(config.Distros, ",")
	}

	// Set CNI
	if config.CNI != "" {
		cc.interactiveSelectedCNI = config.CNI
	}

	// Set source type: community (default) vs prime vs both
	switch strings.ToLower(strings.TrimSpace(config.SourceType)) {
	case "prime-gc", "prime":
		cc.isRPMGC = true
		cc.includeCommunityImageLists = false
	case "both":
		cc.isRPMGC = true
		cc.includeCommunityImageLists = true
	default:
		cc.isRPMGC = false
		cc.includeCommunityImageLists = true
	}

	// Include charts from Application Collection (dp.apps.rancher.io)
	if config.IncludeAppCollectionCharts != nil {
		cc.includeAppCollectionCharts = *config.IncludeAppCollectionCharts
	}
	if config.IncludePartnerCharts != nil {
		cc.includePartnerCharts = *config.IncludePartnerCharts
	}
	if config.IncludeUIPluginCharts != nil {
		cc.includeUIPluginCharts = *config.IncludeUIPluginCharts
	}
	if config.IncludeCertManager != nil {
		cc.includeCertManager = *config.IncludeCertManager
	}

	// Set Windows support (Linux only vs Linux + Windows node images)
	if config.IncludeWindows != nil {
		cc.interactiveIncludeWindows = *config.IncludeWindows
	}
	if config.Arch != "" {
		cc.targetArch = kdmimages.NormalizeLinuxArch(config.Arch)
	}

	// Set load balancer / ingress (K3s: Klipper/Traefik, RKE2: NGINX/Traefik)
	if config.LoadBalancer != nil {
		cc.interactiveIncludeLB = *config.LoadBalancer
		if *config.LoadBalancer {
			cc.interactiveLBK3sKlipper = true
			cc.interactiveLBK3sTraefik = true
			cc.interactiveLBRKE2Nginx = true
			cc.interactiveLBRKE2Traefik = true
		}
	} else {
		cc.interactiveIncludeLB = true
		cc.interactiveLBK3sKlipper = true
		cc.interactiveLBK3sTraefik = true
		cc.interactiveLBRKE2Nginx = true
		cc.interactiveLBRKE2Traefik = true
	}

	// Set versions
	if config.Versions != nil {
		if vers, ok := config.Versions["k3s"]; ok && len(vers) > 0 {
			cc.k3sVersions = strings.Join(vers, ",")
		}
		if vers, ok := config.Versions["rke2"]; ok && len(vers) > 0 {
			cc.rke2Versions = strings.Join(vers, ",")
		}
		if vers, ok := config.Versions["rke"]; ok && len(vers) > 0 {
			cc.rkeVersions = strings.Join(vers, ",")
		}
	}

	// Set groups/charts selection
	if len(config.Groups) > 0 {
		// Groups are handled in Step 2 TUI, store for later use
		cc.interactiveSelectedComponentIDs = config.Groups
	}
	if len(config.Charts) > 0 {
		cc.interactiveSelectedChartNames = config.Charts
		cc.chartsSelection = strings.Join(config.Charts, ",")
	}

	// Set scan configuration
	if config.Scan != nil {
		cc.scan = config.Scan.Enabled
		if config.Scan.Jobs > 0 {
			cc.scanJobs = config.Scan.Jobs
		}
		if config.Scan.Timeout > 0 {
			cc.scanTimeout = config.Scan.Timeout
		}
		if config.Scan.Report != "" {
			cc.scanReport = config.Scan.Report
		}
	}

	logrus.Infof("Loaded configuration from %s", cc.configFile)
	return cc.parseComponentFlags()
}

// applyConfigSelections applies group/chart selections from config file
func (cc *genesisCmd) applyConfigSelections() error {
	if len(cc.interactiveSelectedComponentIDs) == 0 && len(cc.interactiveSelectedChartNames) == 0 {
		// No selections specified, use all
		return nil
	}

	// Build the same data structures as runInteractiveTUI
	sourceGroups := listgenerator.GroupImagesBySource(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	compGroups := listgenerator.GroupImagesByComponent(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	merged := make(map[string]*listgenerator.ComponentGroup)
	for k, v := range sourceGroups {
		merged[k] = v
	}
	for k, v := range compGroups {
		merged[k] = v
	}
	chartGroups := listgenerator.GroupImagesByChart(
		cc.generator.LinuxImages, cc.generator.WindowsImages)

	// Process group selections
	var componentIDs []string
	var chartNames []string

	// Handle "basic" and "addons" groups
	for _, groupID := range cc.interactiveSelectedComponentIDs {
		if groupID == "basic" {
			// Basic group: use BasicPresetWithCNI
			basicIDs := listgenerator.BasicPresetWithCNI(cc.components, cc.interactiveSelectedCNI)
			basicIDs = append(basicIDs, "fleet")
			componentIDs = append(componentIDs, basicIDs...)
		} else if groupID == "addons" {
			// Addons: include all charts
			for name := range chartGroups {
				chartNames = append(chartNames, name)
			}
		} else if groupID == "app_collection" {
			// Application Collection: include its source group (all charts + containers)
			componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollection)
		} else if groupID == "app_collection_containers" || groupID == listgenerator.SourceGroupAppCollectionContainers {
			// Application Collection → Containers only
			componentIDs = append(componentIDs, listgenerator.SourceGroupAppCollectionContainers)
		} else if strings.HasPrefix(groupID, "addon_") {
			// Subgroup like "addon_monitoring"
			category := strings.TrimPrefix(groupID, "addon_")
			for name, cg := range chartGroups {
				if cg.Category == category {
					chartNames = append(chartNames, name)
				}
			}
		} else {
			// Direct component ID
			componentIDs = append(componentIDs, groupID)
		}
	}

	// Add explicit chart selections
	chartNames = append(chartNames, cc.interactiveSelectedChartNames...)

	// Remove duplicates
	seenComponents := make(map[string]bool)
	var uniqueComponents []string
	for _, id := range componentIDs {
		if !seenComponents[id] {
			seenComponents[id] = true
			uniqueComponents = append(uniqueComponents, id)
		}
	}

	seenCharts := make(map[string]bool)
	var uniqueCharts []string
	for _, name := range chartNames {
		if !seenCharts[name] {
			seenCharts[name] = true
			uniqueCharts = append(uniqueCharts, name)
		}
	}

	cc.interactiveSelectedComponentIDs = uniqueComponents
	cc.interactiveSelectedChartNames = uniqueCharts
	if len(uniqueCharts) > 0 {
		cc.chartsSelection = strings.Join(uniqueCharts, ",")
	}

	logrus.Infof("Applied config selections: %d components, %d charts", len(uniqueComponents), len(uniqueCharts))
	return nil
}

// writeSaveConfig writes current TUI selections to a YAML config file (--save-config).
// Includes distros, cni, loadBalancer, versions, groups, charts, and scan settings.
func (cc *genesisCmd) writeSaveConfig() error {
	distros := strings.Split(cc.components, ",")
	for i, d := range distros {
		distros[i] = strings.TrimSpace(d)
	}
	var distrosClean []string
	for _, d := range distros {
		if d != "" {
			distrosClean = append(distrosClean, d)
		}
	}

	versions := make(map[string][]string)
	if cc.k3sVersions != "" && cc.k3sVersions != "all" {
		versions["k3s"] = splitAndTrim(cc.k3sVersions, ",")
	}
	if cc.rke2Versions != "" && cc.rke2Versions != "all" {
		versions["rke2"] = splitAndTrim(cc.rke2Versions, ",")
	}
	if cc.rkeVersions != "" && cc.rkeVersions != "all" {
		versions["rke"] = splitAndTrim(cc.rkeVersions, ",")
	}

	includeLB := cc.interactiveIncludeLB

	sourceType := "community"
	switch {
	case cc.isRPMGC && cc.includeCommunityImageLists:
		sourceType = "both"
	case cc.isRPMGC:
		sourceType = "prime-gc"
	}
	includeAppCollection := cc.includeAppCollectionCharts
	includePartner := cc.includePartnerCharts
	includeUIPlugins := cc.includeUIPluginCharts
	includeCertManager := cc.includeCertManager
	includeWin := cc.interactiveIncludeWindows
	config := generateListConfig{
		Distros:                    distrosClean,
		CNI:                        cc.interactiveSelectedCNI,
		LoadBalancer:               &includeLB,
		IncludeWindows:             &includeWin,
		Versions:                   versions,
		Groups:                     cc.interactiveSelectedComponentIDs,
		Charts:                     cc.interactiveSelectedChartNames,
		SourceType:                 sourceType,
		IncludeAppCollectionCharts: &includeAppCollection,
		IncludePartnerCharts:       &includePartner,
		IncludeUIPluginCharts:      &includeUIPlugins,
		IncludeCertManager:         &includeCertManager,
	}
	if cc.scan {
		config.Scan = &scanConfig{
			Enabled: true,
			Jobs:    cc.scanJobs,
			Timeout: cc.scanTimeout,
			Report:  cc.scanReport,
		}
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	header := "# Generated by hangar genesis --save-config (TUI selections)\n" +
		"# Usage: hangar genesis --rancher=" + cc.rancherVersion + " --config=<path>\n" +
		"# Step 1: distros (k3s, rke2, rke), cni, loadBalancer, versions\n" +
		"# Step 2: groups (basic, addons, addon_*), charts\n"
	if err := os.WriteFile(cc.saveConfigFile, append([]byte(header), data...), 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	logrus.Infof("Saved configuration to %s", cc.saveConfigFile)
	return nil
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// runStep1TUI runs the Step 1 TUI (cluster types + versions + CNI) when --tui.
// It loads KDM to get compatible versions, then runs the TUI and sets components, versions, and CNI.
func (cc *genesisCmd) runStep1TUI() error {
	kdmBytes, err := cc.loadKDMData(signalContext)
	if err != nil {
		return fmt.Errorf("load KDM for Step 1 TUI: %w", err)
	}
	data, err := kdm.FromData(kdmBytes)
	if err != nil {
		return fmt.Errorf("parse KDM data: %w", err)
	}
	minKube := ""
	if cc.minKubeVersion != "" {
		minKube = semver.MajorMinor(cc.minKubeVersion)
	} else {
		switch {
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.7"):
			minKube = "v1.23.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.8"):
			minKube = "v1.25.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.9"):
			minKube = "v1.27.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.10"):
			minKube = "v1.28.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.11"):
			minKube = "v1.30.0"
		default:
			minKube = "v1.30.0"
		}
	}
	capabilities, err := kdmimages.InspectClusterVersions(
		cc.rancherVersion, minKube, cc.kdmRemoveDeprecated, data)
	if err != nil {
		return fmt.Errorf("inspect KDM versions: %w", err)
	}
	hasRKE1 := false
	if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); ok < 0 {
		hasRKE1 = true
	}
	// Step 1: Community vs Rancher Prime (sets cc.isRPMGC)
	isPrimeGC, err := RunSourceTypeTUI()
	if err != nil {
		return err
	}
	cc.isRPMGC = isPrimeGC

	// Second question: include charts from Application Collection (dp.apps.rancher.io)?
	includeAppCollection, err := RunIncludeAppCollectionTUI()
	if err != nil {
		return err
	}
	cc.includeAppCollectionCharts = includeAppCollection
	if cc.includeAppCollectionCharts {
		user, password, err := RunAppCollectionCredentialsTUI()
		if err != nil {
			return fmt.Errorf("Application Collection credentials: %w", err)
		}
		cc.appCollectionAPIUser = user
		cc.appCollectionAPIPassword = password
		logrus.Info("Application Collection enabled; will live-fetch from api.apps.rancher.io. Run 'helm registry login dp.apps.rancher.io' for OCI chart pull.")
	}

	// Build details for Step 1 right panel (KDM URL, image list source)
	details := Step1Details{
		KDMURL:          GetKDMURLForDisplay(cc.rancherVersion, cc.isRPMGC, cc.dev),
		ImageListSource: GetImageListSourceForDisplay(cc.includeCommunityImageLists, cc.isRPMGC),
	}

	// Convert capabilities map to string keys for TUI
	capabilitiesStr := make(map[string]kdmimages.ClusterVersionInfo)
	for k, v := range capabilities {
		capabilitiesStr[string(k)] = v
	}
	components, k3sVers, rke2Vers, rkeVers, cni, lbOpts, includeWindows, err := RunStep1TUI(hasRKE1, capabilitiesStr, details)
	if err != nil {
		return err
	}
	cc.components = components
	cc.k3sVersions = k3sVers
	cc.rke2Versions = rke2Vers
	cc.rkeVersions = rkeVers
	cc.interactiveSelectedCNI = cni
	cc.interactiveLBK3sKlipper = lbOpts.K3sKlipper
	cc.interactiveLBK3sTraefik = lbOpts.K3sTraefik
	cc.interactiveLBRKE2Nginx = lbOpts.RKE2Nginx
	cc.interactiveLBRKE2Traefik = lbOpts.RKE2Traefik
	cc.interactiveIncludeLB = lbOpts.K3sKlipper || lbOpts.K3sTraefik || lbOpts.RKE2Nginx || lbOpts.RKE2Traefik
	cc.interactiveIncludeWindows = includeWindows
	return cc.parseComponentFlags()
}

// loadKDMData resolves the KDM source (path or URL) and returns the KDM JSON
// bytes. It uses the same resolution as prepareGenerator (--kdm or default
// Rancher KDM URL).
func (cc *genesisCmd) loadKDMData(ctx context.Context) ([]byte, error) {
	if cc.kdm != "" {
		if _, err := url.ParseRequestURI(cc.kdm); err != nil {
			return os.ReadFile(cc.kdm)
		}
		client := &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !cc.tlsVerify},
				Proxy:           http.ProxyFromEnvironment,
			},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cc.kdm, nil)
		if err != nil {
			return nil, err
		}
		resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	option := &listgenerator.GeneratorOption{}
	addRancherPrimeKontainerDriverMetadata(cc.rancherVersion, option, cc.dev)
	if option.KDMURL == "" {
		return nil, fmt.Errorf("could not resolve KDM URL for interactive mode")
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !cc.tlsVerify},
			Proxy:           http.ProxyFromEnvironment,
		},
	}
	data, err := fetchKDMWithFallback(ctx, client, cc.rancherVersion, option.KDMURL, cc.dev)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// fetchKDMWithFallback fetches KDM data from primaryURL. If the response is
// 404 and dev was not already requested, it retries with the dev- branch URL.
func fetchKDMWithFallback(ctx context.Context, client *http.Client, version, primaryURL string, devAlreadySet bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, primaryURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := utils.HTTPClientDoWithRetry(ctx, client, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && !devAlreadySet && !shouldUseDev(version, false) {
		resp.Body.Close()
		majorMinor := semver.MajorMinor(version)
		devURL := fmt.Sprintf("%v/dev-%v/data.json", KontainerDriverMetadataURL, majorMinor)
		logrus.Infof("KDM release branch not found (404), falling back to dev branch: %s", devURL)
		req2, err := http.NewRequestWithContext(ctx, http.MethodGet, devURL, nil)
		if err != nil {
			return nil, err
		}
		resp2, err := utils.HTTPClientDoWithRetry(ctx, client, req2)
		if err != nil {
			return nil, err
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("KDM data not available at %s (HTTP %d) or %s (HTTP %d)", primaryURL, http.StatusNotFound, devURL, resp2.StatusCode)
		}
		return io.ReadAll(resp2.Body)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch KDM data from %s: HTTP %d", primaryURL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (cc *genesisCmd) interactivePrompt() error {
	fmt.Println("\n=== Step 2: Cluster types and KDM-backed versions ===")

	// Load KDM and derive compatible versions per cluster type
	kdmBytes, err := cc.loadKDMData(signalContext)
	if err != nil {
		return fmt.Errorf("load KDM for interactive mode: %w", err)
	}
	data, err := kdm.FromData(kdmBytes)
	if err != nil {
		return fmt.Errorf("parse KDM data: %w", err)
	}
	minKube := ""
	if cc.minKubeVersion != "" {
		minKube = semver.MajorMinor(cc.minKubeVersion)
	} else {
		switch {
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.7"):
			minKube = "v1.23.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.8"):
			minKube = "v1.25.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.9"):
			minKube = "v1.27.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.10"):
			minKube = "v1.28.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.11"):
			minKube = "v1.30.0"
		default:
			minKube = "v1.30.0"
		}
	}
	capabilities, err := kdmimages.InspectClusterVersions(
		cc.rancherVersion, minKube, cc.kdmRemoveDeprecated, data)
	if err != nil {
		return fmt.Errorf("inspect KDM versions: %w", err)
	}

	// Prompt for cluster types
	fmt.Println("\nCluster types:")
	fmt.Println("  [1] K3s")
	fmt.Println("  [2] RKE2")
	if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); ok < 0 {
		fmt.Println("  [3] RKE1")
	}
	fmt.Print("Select cluster types (comma-separated, e.g., 1,2): ")
	var clusterInput string
	if _, err := utils.Scanf(signalContext, "%s\n", &clusterInput); err != nil {
		return fmt.Errorf("failed to read cluster type selection: %w", err)
	}
	parts := strings.Split(clusterInput, ",")
	var components []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "1":
			components = append(components, "k3s")
		case "2":
			components = append(components, "rke2")
		case "3":
			if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); ok < 0 {
				components = append(components, "rke")
			}
		}
	}
	cc.components = strings.Join(components, ",")

	// For each selected cluster type, show KDM-derived versions as numbered menu
	cc.k3sVersions = "all"
	cc.rke2Versions = "all"
	cc.rkeVersions = "all"

	if slicesContains(components, "k3s") {
		info, ok := capabilities[kdmimages.K3S]
		if ok && len(info.Versions) > 0 {
			fmt.Println("\nK3s compatible k8s versions (from KDM):")
			for i, v := range info.Versions {
				fmt.Printf("  [%d] %s\n", i+1, v)
			}
			fmt.Print("Select K3s versions (numbers comma-separated, or 'all'): ")
			var k3sInput string
			if _, err := utils.Scanf(signalContext, "%s\n", &k3sInput); err != nil {
				return fmt.Errorf("failed to read K3s versions: %w", err)
			}
			k3sInput = strings.TrimSpace(k3sInput)
			if k3sInput != "" && strings.ToLower(k3sInput) != "all" {
				selected := parseNumberSelection(k3sInput, len(info.Versions))
				if len(selected) > 0 {
					vers := make([]string, 0, len(selected))
					for _, idx := range selected {
						vers = append(vers, info.Versions[idx])
					}
					cc.k3sVersions = strings.Join(vers, ",")
				}
			}
		}
	}
	if slicesContains(components, "rke2") {
		info, ok := capabilities[kdmimages.RKE2]
		if ok && len(info.Versions) > 0 {
			fmt.Println("\nRKE2 compatible k8s versions (from KDM):")
			for i, v := range info.Versions {
				fmt.Printf("  [%d] %s\n", i+1, v)
			}
			fmt.Print("Select RKE2 versions (numbers comma-separated, or 'all'): ")
			var rke2Input string
			if _, err := utils.Scanf(signalContext, "%s\n", &rke2Input); err != nil {
				return fmt.Errorf("failed to read RKE2 versions: %w", err)
			}
			rke2Input = strings.TrimSpace(rke2Input)
			if rke2Input != "" && strings.ToLower(rke2Input) != "all" {
				selected := parseNumberSelection(rke2Input, len(info.Versions))
				if len(selected) > 0 {
					vers := make([]string, 0, len(selected))
					for _, idx := range selected {
						vers = append(vers, info.Versions[idx])
					}
					cc.rke2Versions = strings.Join(vers, ",")
				}
			}
		}
	}
	if slicesContains(components, "rke") {
		info, ok := capabilities[kdmimages.RKE]
		if ok && len(info.Versions) > 0 {
			fmt.Println("\nRKE1 compatible k8s versions (from KDM):")
			for i, v := range info.Versions {
				fmt.Printf("  [%d] %s\n", i+1, v)
			}
			fmt.Print("Select RKE1 versions (numbers comma-separated, or 'all'): ")
			var rkeInput string
			if _, err := utils.Scanf(signalContext, "%s\n", &rkeInput); err != nil {
				return fmt.Errorf("failed to read RKE1 versions: %w", err)
			}
			rkeInput = strings.TrimSpace(rkeInput)
			if rkeInput != "" && strings.ToLower(rkeInput) != "all" {
				selected := parseNumberSelection(rkeInput, len(info.Versions))
				if len(selected) > 0 {
					vers := make([]string, 0, len(selected))
					for _, idx := range selected {
						vers = append(vers, info.Versions[idx])
					}
					cc.rkeVersions = strings.Join(vers, ",")
				}
			}
		}
	}

	// Step 1: CNI selection (for Standard preset and pre-select)
	fmt.Println("\nCNI (for Standard preset and filtering):")
	fmt.Println("  [1] Canal")
	fmt.Println("  [2] Calico")
	fmt.Println("  [3] Flannel")
	fmt.Println("  [4] All CNI")
	fmt.Println("  [5] None")
	fmt.Print("Select CNI (1–5, default 4): ")
	var cniInput string
	if _, err := utils.Scanf(signalContext, "%s\n", &cniInput); err != nil {
		return fmt.Errorf("failed to read CNI selection: %w", err)
	}
	cniInput = strings.TrimSpace(cniInput)
	switch cniInput {
	case "1":
		cc.interactiveSelectedCNI = "cni_canal"
	case "2":
		cc.interactiveSelectedCNI = "cni_calico"
	case "3":
		cc.interactiveSelectedCNI = "cni_flannel"
	case "5":
		cc.interactiveSelectedCNI = ""
	case "4", "":
		cc.interactiveSelectedCNI = "cni"
	default:
		cc.interactiveSelectedCNI = "cni"
	}

	cc.chartsSelection = "all"
	return cc.parseComponentFlags()
}

func slicesContains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// parseNumberSelection parses a comma-separated list of 1-based indices and
// returns the 0-based indices (validated against maxN). Invalid entries are skipped.
func parseNumberSelection(input string, maxN int) []int {
	var out []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > maxN {
			continue
		}
		out = append(out, n-1)
	}
	return out
}

func (cc *genesisCmd) interactivePostRunPrompt() error {
	if cc.tui {
		return cc.runInteractiveTUI()
	}
	return cc.runInteractiveText()
}

// runInteractiveText shows one combined list (presets + groups + charts) and one prompt.
func (cc *genesisCmd) runInteractiveText() error {
	sourceGroups := listgenerator.GroupImagesBySource(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	compGroups := listgenerator.GroupImagesByComponent(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	merged := make(map[string]*listgenerator.ComponentGroup)
	for k, v := range sourceGroups {
		merged[k] = v
	}
	for k, v := range compGroups {
		merged[k] = v
	}
	chartGroups := listgenerator.GroupImagesByChart(
		cc.generator.LinuxImages, cc.generator.WindowsImages)

	type row struct {
		kind  string // "preset", "component", "chart_all", "chart"
		id    string
		label string
		count int
		desc  string
	}
	var rows []row
	// 1=Min, 2=Standard, 3=Full
	rows = append(rows, row{"preset", "1", "Minimum Viable (KDM + System Add-ons)", 0, ""})
	rows = append(rows, row{"preset", "2", "Essentials (Rancher components + RKE2 Essentials + CNI)", 0, ""})
	rows = append(rows, row{"preset", "3", "Full Stack (+ Monitoring, Logging, Backup)", 0, ""})
	// Four groups only: K3s core, RKE2 core, RKE1 core, Charts (expand for chart list)
	for _, id := range []string{listgenerator.SourceGroupK3s, listgenerator.SourceGroupRKE2, listgenerator.SourceGroupRKE1} {
		if g := merged[id]; g != nil && g.Count() > 0 {
			rows = append(rows, row{"component", id, g.Name, g.Count(), g.Description})
		}
	}
	if g := merged[listgenerator.SourceGroupCharts]; g != nil && g.Count() > 0 {
		rows = append(rows, row{"chart_all", "", "Charts (all)", g.Count(), g.Description})
	}
	// Individual charts (sorted)
	var chartNames []string
	for name := range chartGroups {
		chartNames = append(chartNames, name)
	}
	sort.Strings(chartNames)
	for _, name := range chartNames {
		g := chartGroups[name]
		c := 0
		if g != nil {
			c = g.Count()
		}
		cat := ""
		if g != nil && g.Category != "" {
			cat = " [" + g.Category + "]"
		}
		rows = append(rows, row{"chart", name, name + cat, c, ""})
	}

	fmt.Println("\n=== Step 3: What to include (groups) ===")
	fmt.Println("Tip: use --scan to add vulnerability scan to each image in the output.")
	fmt.Println("Presets (pick one for a bundle):")
	fmt.Println("  [1] Minimum Viable — KDM core + System Add-ons")
	stdCni := cc.interactiveSelectedCNI
	if stdCni == "" {
		stdCni = "none"
	}
	basicDesc := "Essentials — Rancher components"
	if strings.Contains(cc.components, "k3s") || strings.Contains(cc.components, "1") {
		basicDesc += " + K3s"
	}
	if strings.Contains(cc.components, "rke2") || strings.Contains(cc.components, "2") {
		basicDesc += " + RKE2"
	}
	if strings.Contains(cc.components, "rke") || strings.Contains(cc.components, "3") {
		basicDesc += " + RKE1"
	}
	if stdCni != "none" {
		basicDesc += " + CNI (" + stdCni + ")"
	}
	fmt.Println("  [2] " + basicDesc)
	fmt.Println("  [3] Full Stack — + Monitoring, Logging, Backup")
	fmt.Println("Groups (expand 4 for charts):")
	for i, r := range rows {
		if r.kind == "preset" {
			continue
		}
		if r.count > 0 {
			fmt.Printf("  [%d] %s (%d images)", i+1, r.label, r.count)
			if r.desc != "" {
				fmt.Printf(" — %s", r.desc)
			}
			fmt.Println()
		} else {
			fmt.Printf("  [%d] %s\n", i+1, r.label)
		}
	}
	fmt.Println()
	fmt.Print("What to type:  1 or 2 or 3 = preset bundle   |   4,5,6,... = group/chart numbers   |   all = everything: ")
	var input string
	if _, err := utils.Scanf(signalContext, "%s\n", &input); err != nil {
		return fmt.Errorf("failed to read selection: %w", err)
	}
	input = strings.TrimSpace(input)
	if input == "" || strings.ToLower(input) == "all" {
		return nil
	}
	selected := parseNumberSelection(input, len(rows))
	presetOnly := true
	for _, idx := range selected {
		if idx >= len(rows) {
			continue
		}
		r := rows[idx]
		if r.kind != "preset" {
			presetOnly = false
			break
		}
	}
	if presetOnly && len(selected) == 1 && selected[0] < 3 {
		switch rows[selected[0]].id {
		case "1":
			cc.interactiveSelectedComponentIDs = listgenerator.PriorityLevel1Preset()
		case "2":
			cc.interactiveSelectedComponentIDs = listgenerator.BasicPresetWithCNI(cc.components, cc.interactiveSelectedCNI)
		case "3":
			cc.interactiveSelectedComponentIDs = listgenerator.PriorityLevel3Preset()
		}
		return nil
	}
	for _, idx := range selected {
		if idx >= len(rows) {
			continue
		}
		r := rows[idx]
		switch r.kind {
		case "preset":
			if len(cc.interactiveSelectedComponentIDs) == 0 {
				switch r.id {
				case "1":
					cc.interactiveSelectedComponentIDs = listgenerator.PriorityLevel1Preset()
				case "2":
					cc.interactiveSelectedComponentIDs = listgenerator.StandardPresetWithCNI(cc.interactiveSelectedCNI)
				case "3":
					cc.interactiveSelectedComponentIDs = listgenerator.PriorityLevel3Preset()
				}
			}
		case "component":
			cc.interactiveSelectedComponentIDs = append(cc.interactiveSelectedComponentIDs, r.id)
		case "chart_all":
			// Include all charts (no chart filter)
		case "chart":
			cc.interactiveSelectedChartNames = append(cc.interactiveSelectedChartNames, r.id)
		}
	}
	return nil
}

// inferChartFromImage maps an image ref to the RKE2/K3s/Rancher Helm chart
// that deploys it. These charts are bundled inside the distro binary and
// visible as separate image lists on the RKE2/K3s GitHub release pages
// (e.g. rke2-images-calico.txt, rke2-images-core.txt, etc.).
func inferChartFromImage(img, componentLabel string) string {
	l := strings.ToLower(img)
	name := ""
	if idx := strings.LastIndex(l, "/"); idx >= 0 {
		name = l[idx+1:]
	} else {
		name = l
	}
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[:i]
	}

	switch componentLabel {
	case "RKE2":
		switch {
		case strings.Contains(name, "coredns"):
			return "rke2-coredns"
		case strings.Contains(name, "metrics-server"):
			return "rke2-metrics-server"
		case strings.Contains(name, "snapshot-controller") || strings.Contains(name, "csi-snapshotter"):
			return "rke2-snapshot-controller"
		case strings.Contains(name, "cluster-autoscaler"):
			return "rke2-cluster-autoscaler"
		case strings.Contains(name, "addon-resizer"):
			return "rke2-metrics-server"
		case strings.Contains(name, "dns-node-cache"):
			return "rke2-dns-node-cache"
		case strings.Contains(name, "etcd"):
			return "rke2-etcd"
		case strings.Contains(name, "multus") || strings.Contains(name, "whereabouts"):
			return "rke2-multus"
		case strings.Contains(name, "cloud-provider") && strings.Contains(name, "vsphere"):
			return "rke2-vsphere-cpi"
		case strings.Contains(name, "vsphere") || strings.Contains(name, "csi-release"):
			return "rke2-vsphere-csi"
		case strings.Contains(name, "rke2-cloud-provider"):
			return "rke2-cloud-provider"
		case strings.Contains(name, "harvester-cloud"):
			return "harvester-cloud-provider"
		case strings.Contains(name, "harvester-csi"):
			return "harvester-csi-driver"
		case strings.Contains(name, "longhornio") || strings.Contains(name, "longhorn"):
			return "longhorn-csi"
		case strings.Contains(name, "klipper-helm"):
			return "rke2-helm-controller"
		case strings.Contains(name, "klipper-lb"):
			return "klipper-lb"
		case strings.Contains(name, "kube-proxy"):
			return "rke2-kube-proxy"
		case strings.Contains(name, "kube-vip"):
			return "rke2-kube-vip"
		case strings.Contains(name, "sig-storage"):
			return "rke2-snapshot-controller"
		case strings.Contains(name, "pause"):
			return "rke2-runtime"
		case strings.Contains(name, "kubernetes") || strings.Contains(name, "rke2-runtime"):
			return "rke2-runtime"
		default:
			return "rke2-core"
		}
	case "K3s":
		switch {
		case strings.Contains(name, "coredns"):
			return "k3s-coredns"
		case strings.Contains(name, "metrics-server"):
			return "k3s-metrics-server"
		case strings.Contains(name, "local-path"):
			return "k3s-local-path-provisioner"
		case strings.Contains(name, "traefik"):
			return "k3s-traefik"
		case strings.Contains(name, "klipper-lb"):
			return "k3s-klipper-lb"
		case strings.Contains(name, "klipper-helm"):
			return "k3s-helm-controller"
		default:
			return "k3s-runtime"
		}
	case "CNI":
		switch {
		case strings.Contains(name, "calico"):
			return "rke2-calico"
		case strings.Contains(name, "canal"):
			return "rke2-canal"
		case strings.Contains(name, "flannel"):
			return "rke2-canal"
		case strings.Contains(name, "cilium"):
			return "rke2-cilium"
		case strings.Contains(name, "multus") || strings.Contains(name, "whereabouts"):
			return "rke2-multus"
		case strings.Contains(name, "cni-plugins"):
			return "cni-plugins"
		default:
			return "cni-plugins"
		}
	case "Load Balancer / Ingress":
		switch {
		case strings.Contains(name, "nginx") || strings.Contains(name, "ingress-nginx"):
			return "rke2-ingress-nginx"
		case strings.Contains(name, "traefik"):
			return "traefik"
		case strings.Contains(name, "klipper"):
			return "klipper-lb"
		default:
			return "rke2-ingress-nginx"
		}
	case "Rancher":
		switch {
		case strings.Contains(name, "fleet"):
			return "fleet"
		case strings.Contains(name, "webhook"):
			return "rancher-webhook"
		case strings.Contains(name, "turtles"):
			return "rancher-turtles"
		case strings.Contains(name, "shell"):
			return "rancher-shell"
		case strings.Contains(name, "system-upgrade"):
			return "system-upgrade-controller"
		case strings.Contains(name, "machine"):
			return "rancher-machine"
		case strings.Contains(name, "rancher-agent") || name == "rancher":
			return "rancher"
		default:
			return "rancher"
		}
	case "Cert Manager":
		return "cert-manager"
	}
	return "other"
}

// inferChartVersion extracts the most representative version from a chart's image tags.
func inferChartVersion(imgs []string) string {
	versions := make(map[string]int)
	for _, img := range imgs {
		tag := ""
		if i := strings.LastIndex(img, ":"); i >= 0 {
			tag = img[i+1:]
		}
		if tag == "" {
			continue
		}
		// Strip build suffixes like "-build20260119", "-hardened1"
		ver := tag
		if i := strings.Index(ver, "-build"); i > 0 {
			ver = ver[:i]
		}
		if i := strings.Index(ver, "-hardened"); i > 0 {
			ver = ver[:i]
		}
		// Only keep semver-like versions
		if len(ver) > 0 && (ver[0] == 'v' || (ver[0] >= '0' && ver[0] <= '9')) {
			versions[ver]++
		}
	}
	if len(versions) == 0 {
		return ""
	}
	// Return the most common version, or the highest if tied
	best := ""
	bestCount := 0
	for v, c := range versions {
		if c > bestCount || (c == bestCount && v > best) {
			best = v
			bestCount = c
		}
	}
	return best
}

// basicComponentDescriptions explains Essentials subgroups in the Step 3 tree.
var basicComponentDescriptions = map[string]string{
	"Rancher":                 "Core Rancher server, agent, webhooks, Fleet GitOps, Rancher Turtles (CAPI), and auto-deployed system charts.",
	"CNI":                     "Container network interface (Calico, Canal, Flannel, etc.) — pod networking for your cluster.",
	"K3s":                     "Lightweight Kubernetes distribution images bundled with your selected K3s version(s).",
	"RKE2":                    "RKE2 (Rancher Kubernetes Engine 2) node and system images for your selected version(s).",
	"RKE1":                    "Legacy RKE1 cluster provisioning images (when RKE1 is selected).",
	"Load Balancer / Ingress": "Ingress controller or load-balancer images (nginx, Traefik, Klipper) for exposing services.",
}

// chartCategoryDescriptions explains AddOn chart categories.
var chartCategoryDescriptions = map[string]string{
	"monitoring":     "Prometheus, Grafana, Alertmanager — cluster observability and metrics.",
	"logging":        "Fluent Bit / Fluentd log collectors and forwarding.",
	"backup-restore": "Velero and backup-restore operators for cluster disaster recovery.",
	"storage":        "Longhorn, Harvester, CSI drivers, and persistent storage operators.",
	"security":       "NeuVector, Gatekeeper, and runtime security tooling.",
	"cis":            "CIS benchmark scanning and compliance reporting.",
	"provisioning":   "Cloud provider operators for EKS, GKE, AKS, vSphere.",
	"networking":     "Service mesh (Istio), SR-IOV, and advanced networking add-ons.",
	"cluster-api":    "Optional upstream CAPI infrastructure providers (CAPA, CAPV, …) via CAPIProvider — not Rancher Turtles itself.",
	"os-management":  "Elemental and edge/OS lifecycle management.",
	"support":        "Supportability and diagnostic tools.",
	"other":          "Additional marketplace charts not in a named category.",
	"fleet":          "Fleet GitOps — deploy applications and multi-cluster workloads from Git.",
	"system":         "Charts auto-deployed by Rancher (webhook, Fleet, Turtles/CAPI, system-upgrade, cert-manager).",
	"cert-manager":   "TLS certificate controller required before installing Rancher with Helm.",
}

func refsToTreeNodes(refs []string) []treeNode {
	nodes := make([]treeNode, 0, len(refs))
	for _, ref := range refs {
		nodes = append(nodes, treeNode{
			Id: ref, Label: ref, Kind: "image", Count: 0,
			Version: imageTagFromRef(ref),
		})
	}
	return nodes
}

func imageTagFromRef(ref string) string {
	if i := strings.LastIndex(ref, ":"); i > 0 {
		return ref[i+1:]
	}
	return ""
}

// applyChartMetadata enriches a chart tree node with icon URL and description
// collected from the Helm repo index (when available).
func (cc *genesisCmd) applyChartMetadata(node *treeNode, chartName string) {
	if cc.generator == nil {
		return
	}
	meta, ok := cc.generator.ChartMetadata[chartName]
	if !ok {
		return
	}
	node.IconURL = meta.IconURL
	if node.Description == "" {
		node.Description = meta.Description
	}
	if node.Version == "" {
		node.Version = meta.Version
	}
}

// chartRepoTypeForSourceGroup maps a source group ID to the ChartMetadata.Repo
// string collected while fetching charts (see chartimages.ChartMetadata.Repo).
func chartRepoTypeForSourceGroup(repoID string) string {
	switch repoID {
	case listgenerator.SourceGroupPartnerCharts:
		return "partner"
	case listgenerator.SourceGroupUIPluginCharts:
		return "ui-plugins"
	default:
		return ""
	}
}

// buildChartRepoRoot builds a tree root for one opt-in chart repo (partner
// charts, UI plugin charts) from the chart groups whose Repo matches repoID.
// Charts with no container images (typical for UI plugins) are included from
// ChartMetadata when the repo was fetched. Returns ok=false when the repo
// contributed no charts.
func (cc *genesisCmd) buildChartRepoRoot(
	chartGroups map[string]*listgenerator.ChartComponentGroup,
	chartNamesSorted []string,
	repoID, label, description string,
) (treeNode, bool) {
	var chartNodes []treeNode
	totalImgs := 0
	seen := make(map[string]bool)
	for _, name := range chartNamesSorted {
		cg := chartGroups[name]
		if cg == nil || cg.Repo != repoID {
			continue
		}
		var imgs []string
		for img := range cg.LinuxImages {
			imgs = append(imgs, img)
		}
		for img := range cg.WindowsImages {
			if !cg.LinuxImages[img] {
				imgs = append(imgs, img)
			}
		}
		sort.Strings(imgs)
		ver := inferChartVersion(imgs)
		verLabel := ""
		if ver != "" {
			verLabel = " " + ver
		}
		node := treeNode{
			Id: name, Label: name + verLabel,
			Kind: "chart", Count: cg.Count(),
			Version: ver, Category: cg.Category,
			Children: refsToTreeNodes(imgs),
		}
		cc.applyChartMetadata(&node, name)
		chartNodes = append(chartNodes, node)
		totalImgs += cg.Count()
		seen[name] = true
	}
	// UI plugin charts (and some partner charts) may have no container images;
	// include them from Helm index metadata so the opt-in group still appears.
	if cc.generator != nil {
		metaRepoType := chartRepoTypeForSourceGroup(repoID)
		if metaRepoType != "" {
			var metaNames []string
			for name, meta := range cc.generator.ChartMetadata {
				if meta.Repo == metaRepoType && !seen[name] {
					metaNames = append(metaNames, name)
				}
			}
			sort.Strings(metaNames)
			for _, name := range metaNames {
				meta := cc.generator.ChartMetadata[name]
				verLabel := ""
				if meta.Version != "" {
					verLabel = " " + meta.Version
				}
				node := treeNode{
					Id: name, Label: name + verLabel,
					Kind: "chart", Count: 0,
					Version: meta.Version, Description: meta.Description,
					IconURL: meta.IconURL, Children: nil,
				}
				chartNodes = append(chartNodes, node)
				seen[name] = true
			}
		}
	}
	if len(chartNodes) == 0 {
		return treeNode{}, false
	}
	return treeNode{
		Id:    repoID,
		Label: label + " (" + strconv.Itoa(len(chartNodes)) + " charts, " + strconv.Itoa(totalImgs) + " images)",
		Kind:  "component", Count: totalImgs,
		Description: description,
		Children:    chartNodes,
	}, true
}

// buildGenesisTree builds the tree for Step 3 (TUI or API). Returns roots, basicCharts, fleetCharts, cniCharts, basicImageComponent, pastSelection.
func (cc *genesisCmd) buildGenesisTree() (roots []treeNode, basicCharts []treeNode, fleetCharts []treeNode, cniCharts []treeNode, basicImageComponent map[string]string, pastSelection string) {
	sourceGroups := listgenerator.GroupImagesBySource(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	compGroups := listgenerator.GroupImagesByComponent(
		cc.generator.LinuxImages, cc.generator.WindowsImages)
	merged := make(map[string]*listgenerator.ComponentGroup)
	for k, v := range sourceGroups {
		merged[k] = v
	}
	for k, v := range compGroups {
		merged[k] = v
	}
	chartGroups := listgenerator.GroupImagesByChart(
		cc.generator.LinuxImages, cc.generator.WindowsImages)

	linux := cc.generator.LinuxImages
	windows := cc.generator.WindowsImages

	// Build functional groups (CNI, Fleet, etc.) with their charts first
	functionalGroups := map[string][]string{
		"cni":   []string{}, // CNI charts
		"fleet": []string{}, // Fleet charts
	}
	var chartNamesSorted []string
	for name := range chartGroups {
		chartNamesSorted = append(chartNamesSorted, name)
	}
	sort.Strings(chartNamesSorted)
	for _, name := range chartNamesSorted {
		cg := chartGroups[name]
		if cg == nil {
			continue
		}
		if cg.Category == "fleet" || strings.Contains(name, "fleet") {
			functionalGroups["fleet"] = append(functionalGroups["fleet"], name)
		}
		// CNI charts are typically not in charts, but we can check
		if strings.Contains(name, "calico") || strings.Contains(name, "flannel") || strings.Contains(name, "canal") || strings.Contains(name, "cni") {
			functionalGroups["cni"] = append(functionalGroups["cni"], name)
		}
	}

	// Note: CNI and Fleet are now included directly in Basic group images
	// No need to build separate group nodes since Basic is flat

	// Build tree: Only 2 groups - Basic and AddOns
	roots = nil

	// Group 1: Basic (Rancher components + selected distro + preselected CNI + Fleet)
	// Basic should contain ALL images directly (flat structure, no sub-groups)
	selectedCNIs := listgenerator.SelectedCNISet(cc.interactiveSelectedCNI)
	var basicIDs []string
	if len(selectedCNIs) > 0 {
		basicIDs = listgenerator.BasicPresetWithCNIs(cc.components, keysOf(selectedCNIs))
	} else {
		basicIDs = listgenerator.BasicPresetWithCNI(cc.components, cc.interactiveSelectedCNI)
	}
	// Add Fleet to Basic
	basicIDs = append(basicIDs, "fleet")

	// Collect ALL images from Basic (distro + CNI + Rancher + Fleet)
	// FilterImageSetsBySelection will only include the selected CNI (e.g., cni_calico) if that's what was selected
	linuxBasic, winBasic := listgenerator.FilterImageSetsBySelection(linux, windows, basicIDs, nil)
	basicImgs := imageRefsFromMaps(linuxBasic, winBasic)
	// Filter images by selected Kubernetes versions
	basicImgs = filterImagesByVersions(basicImgs, cc.k3sVersions, cc.rke2Versions, cc.rkeVersions)

	// CRITICAL: Filter out images that don't belong in Basic
	// Basic should only contain: Rancher components, distro core, selected CNI, Fleet
	// Exclude: storage, cloud providers, monitoring, logging, backup, etc.
	// Get component groups to identify excluded components
	compGroupsForFilter := listgenerator.GroupImagesByComponent(linux, windows)
	var filteredBasicImgs []string

	// Define component groups that should be EXCLUDED from Basic
	excludedFromBasic := map[string]bool{
		"longhorn":       true, // Storage
		"backup-restore": true, // Backup
		"monitoring":     true, // Monitoring & Observability
		"logging":        true, // Logging
		"cis":            true, // CIS Benchmark & Compliance
		"neuvector":      true, // Security
		"gatekeeper":     true, // Security
		"provisioning":   true, // Cloud Provider Operators (AKS, EKS, GKE, Ali)
	}

	for _, img := range basicImgs {
		shouldExclude := false

		// 1. Filter out OTHER CNIs if specific CNI(s) were selected
		if len(selectedCNIs) > 0 {
			isOtherCNI := false
			// Check if image is in a specific CNI group that was NOT selected
			for _, cniID := range []string{"cni_canal", "cni_flannel", "cni_calico", "cni_cilium"} {
				if selectedCNIs[cniID] {
					continue
				}
				if g := compGroupsForFilter[cniID]; g != nil {
					if g.LinuxImages[img] || g.WindowsImages[img] {
						isOtherCNI = true
						break
					}
				}
			}
			// Also exclude generic "cni" group images unless the image is in one of
			// our selected specific CNI groups.
			if !isOtherCNI {
				if g := compGroupsForFilter["cni"]; g != nil {
					if g.LinuxImages[img] || g.WindowsImages[img] {
						inSelected := false
						for cniID := range selectedCNIs {
							if sg := compGroupsForFilter[cniID]; sg != nil && (sg.LinuxImages[img] || sg.WindowsImages[img]) {
								inSelected = true
								break
							}
						}
						if !inSelected {
							isOtherCNI = true
						}
					}
				}
			}
			if isOtherCNI {
				shouldExclude = true
			}
		}

		// 2. Filter out images that belong to excluded component groups
		// (even if they have distro source tags, they shouldn't be in Basic)
		for compID := range excludedFromBasic {
			if g := compGroupsForFilter[compID]; g != nil {
				if g.LinuxImages[img] || g.WindowsImages[img] {
					shouldExclude = true
					break
				}
			}
		}

		// 3. Filter out CNI images by name pattern if specific CNI(s) were selected
		// (catches CNI images with distro source tags that bypass component filtering)
		imgLower := strings.ToLower(img)
		if len(selectedCNIs) > 0 {
			// Build the set of CNI name tokens that ARE selected; exclude images
			// whose name contains a CNI token that is NOT selected.
			selectedTokens := map[string]bool{}
			if selectedCNIs["cni_canal"] {
				selectedTokens["canal"] = true
			}
			if selectedCNIs["cni_calico"] {
				selectedTokens["calico"] = true
			}
			if selectedCNIs["cni_cilium"] {
				selectedTokens["cilium"] = true
			}
			if selectedCNIs["cni_flannel"] {
				selectedTokens["flannel"] = true
			}
			for _, tok := range []string{"canal", "calico", "cilium", "flannel"} {
				if selectedTokens[tok] {
					continue
				}
				if strings.Contains(imgLower, tok) {
					shouldExclude = true
					break
				}
			}
		}

		// 4. Optional storage / infra cloud providers (not distro defaults)
		if excludeOptionalStorageFromEssentials(imgLower) {
			shouldExclude = true
		}

		if !shouldExclude {
			filteredBasicImgs = append(filteredBasicImgs, img)
		}
	}
	basicImgs = filteredBasicImgs

	// Add images from Rancher core system charts (default Helm charts deployed by Rancher).
	// These are always part of Basic even if not in component-based basicImgs.
	// Rancher ≥2.14 removed embedded provisioning-capi; Turtles is the CAPI integration.
	// Rancher ≥2.13 enables Turtles by default — keep it in Essentials for all versions we care about.
	coreSystemChartNames := []string{
		"rancher",                   // Main Rancher server Helm chart (rancher-rancher)
		"rancher-rancher",           // Main Rancher server Helm chart (alternate name)
		"rancher-webhook",           // Admission webhooks for Rancher resources
		"rancher-turtles",           // CAPI extension (default 2.13+; only path in 2.14+)
		"system-upgrade-controller", // Manages system upgrades
		"remotedialer-proxy",        // Proxy for remote dialer connections
		"cert-manager",              // Prerequisite for Rancher Helm install (TLS)
	}
	if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.14.0-0"); ok < 0 {
		// Legacy embedded Cluster API (removed in Rancher 2.14)
		coreSystemChartNames = append(coreSystemChartNames, "rancher-provisioning-capi")
	}
	basicImgSet := make(map[string]bool)
	for _, img := range basicImgs {
		basicImgSet[img] = true
	}
	for _, chartName := range coreSystemChartNames {
		cg := chartGroups[chartName]
		if cg == nil {
			continue
		}
		for img := range cg.LinuxImages {
			if !basicImgSet[img] {
				basicImgSet[img] = true
				basicImgs = append(basicImgs, img)
			}
		}
		for img := range cg.WindowsImages {
			if !basicImgSet[img] {
				basicImgSet[img] = true
				basicImgs = append(basicImgs, img)
			}
		}
	}

	// Add well-known core Rancher images that may not appear in KDM/chart sources
	// (e.g. rancher/rancher - main Rancher server); one image per Rancher version when multiple are selected
	var wellKnownCoreImages []string
	if len(cc.rancherVersionsList) > 0 {
		for _, tag := range cc.rancherVersionsList {
			if tag == "" {
				tag = "latest"
			}
			wellKnownCoreImages = append(wellKnownCoreImages, "rancher/rancher:"+tag)
		}
	} else {
		rancherTag := cc.rancherVersion
		if rancherTag == "" {
			rancherTag = "latest"
		}
		wellKnownCoreImages = []string{"rancher/rancher:" + rancherTag}
	}
	for _, img := range wellKnownCoreImages {
		if !basicImgSet[img] {
			basicImgSet[img] = true
			basicImgs = append(basicImgs, img)
		}
	}
	sort.Strings(basicImgs)

	// Exclude load balancer images per Step 1 choices (K3s: Klipper/Traefik; RKE2: NGINX/Traefik)
	includeKlipper := cc.interactiveIncludeLB && cc.interactiveLBK3sKlipper
	includeK3sTraefik := cc.interactiveIncludeLB && cc.interactiveLBK3sTraefik
	includeRKE2Nginx := cc.interactiveIncludeLB && cc.interactiveLBRKE2Nginx
	includeRKE2Traefik := cc.interactiveIncludeLB && cc.interactiveLBRKE2Traefik
	var filteredLB []string
	for _, img := range basicImgs {
		imgLower := strings.ToLower(img)
		if strings.Contains(imgLower, "klipper-helm") || strings.Contains(imgLower, "klipper-lb") {
			if !includeKlipper {
				continue
			}
		} else if strings.Contains(imgLower, "nginx-ingress") || strings.Contains(imgLower, "ingress-nginx") || strings.Contains(imgLower, "mirrored-ingress-nginx") {
			if !includeRKE2Nginx {
				continue
			}
		} else if strings.Contains(imgLower, "traefik") {
			if !includeK3sTraefik && !includeRKE2Traefik {
				continue
			}
		}
		filteredLB = append(filteredLB, img)
	}
	basicImgs = filteredLB
	sort.Strings(basicImgs)

	// Classify each Basic image for the legend (R=Rancher, F=Fleet, C=CNI, D=Distro, LB=Load balancer; single LB group)
	basicImageComponent = make(map[string]string)
	componentPriority := []string{"system_addons", "fleet", "cni_canal", "cni_calico", "cni_flannel", "cni_cilium", "cni", listgenerator.SourceGroupK3s, listgenerator.SourceGroupRKE2, listgenerator.SourceGroupRKE1}
	componentLabels := map[string]string{
		"system_addons": "Rancher", listgenerator.SourceGroupK3s: "K3s", listgenerator.SourceGroupRKE2: "RKE2", listgenerator.SourceGroupRKE1: "RKE1",
		"fleet": "Fleet", "cni": "CNI", "cni_canal": "CNI", "cni_calico": "CNI", "cni_flannel": "CNI", "cni_cilium": "CNI",
	}
	for _, img := range basicImgs {
		imgLower := strings.ToLower(img)
		if strings.Contains(imgLower, "klipper-helm") || strings.Contains(imgLower, "klipper-lb") ||
			strings.Contains(imgLower, "nginx-ingress") || strings.Contains(imgLower, "ingress-nginx") ||
			strings.Contains(imgLower, "traefik") {
			basicImageComponent[img] = "Load Balancer / Ingress"
			continue
		}
		if strings.Contains(imgLower, "cert-manager") {
			basicImageComponent[img] = "Cert Manager"
			continue
		}
		if strings.Contains(imgLower, "rancher/rancher:") {
			basicImageComponent[img] = "Rancher"
			continue
		}
		for _, compID := range componentPriority {
			g := merged[compID]
			if g == nil {
				continue
			}
			if g.LinuxImages[img] || g.WindowsImages[img] {
				if label, ok := componentLabels[compID]; ok {
					basicImageComponent[img] = label
				} else {
					basicImageComponent[img] = compID
				}
				break
			}
		}
		if basicImageComponent[img] == "" {
			compParts := strings.Split(cc.components, ",")
			if len(compParts) == 1 {
				switch strings.TrimSpace(compParts[0]) {
				case "k3s":
					basicImageComponent[img] = "K3s"
				case "rke2":
					basicImageComponent[img] = "RKE2"
				case "rke":
					basicImageComponent[img] = "RKE1"
				default:
					basicImageComponent[img] = "RKE2"
				}
			} else {
				basicImageComponent[img] = "RKE2"
			}
		}
	}

	// Build Essentials tree: group images by their chart (inferred from image name).
	// RKE2/K3s bundle Helm charts internally; we map images to chart names.
	var basicChildren []treeNode

	// Map each image to a chart name within its component group
	type chartBucket struct {
		chart  string
		images []string
	}
	buildChartSubgroups := func(label string, imgs []string) treeNode {
		byChart := make(map[string][]string)
		for _, img := range imgs {
			ch := inferChartFromImage(img, label)
			byChart[ch] = append(byChart[ch], img)
		}
		var chartNames []string
		for ch := range byChart {
			chartNames = append(chartNames, ch)
		}
		sort.Strings(chartNames)
		var chartChildren []treeNode
		for _, ch := range chartNames {
			cImgs := byChart[ch]
			sort.Strings(cImgs)
			ver := inferChartVersion(cImgs)
			chartLabel := ch
			if ver != "" {
				chartLabel = ch + " " + ver
			}
			chartChildren = append(chartChildren, treeNode{
				Id: "basic_chart_" + ch, Label: chartLabel,
				Kind: "chart", Count: len(cImgs),
				Version:  ver,
				Children: refsToTreeNodes(cImgs),
			})
		}
		id := "basic_" + strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(label, " ", "_"), "/", ""))
		return treeNode{
			Id: id, Label: label + " (" + strconv.Itoa(len(imgs)) + " images, " + strconv.Itoa(len(chartChildren)) + " charts)",
			Kind: "component", Count: len(imgs),
			Description: basicComponentDescriptions[label],
			Children:    chartChildren,
		}
	}

	// Assign images to component groups (Rancher, CNI, RKE2/K3s, LB)
	type compGroup struct {
		label  string
		images []string
	}
	groupOrder := []string{"Rancher", "CNI", "K3s", "RKE2", "RKE1", "Load Balancer / Ingress"}
	byGroup := make(map[string][]string)
	for _, img := range basicImgs {
		label := basicImageComponent[img]
		if label == "" || label == "Distro" {
			compParts := strings.Split(cc.components, ",")
			if len(compParts) == 1 {
				switch strings.TrimSpace(compParts[0]) {
				case "k3s":
					label = "K3s"
				case "rke2":
					label = "RKE2"
				case "rke":
					label = "RKE1"
				default:
					label = "RKE2"
				}
			} else {
				label = "RKE2"
			}
		}
		if label == "Fleet" {
			label = "Rancher"
		}
		if label == "Load Balancer / Ingress" || label == "LB" {
			label = "Load Balancer / Ingress"
		}
		byGroup[label] = append(byGroup[label], img)
	}
	for _, label := range groupOrder {
		imgs := byGroup[label]
		if len(imgs) == 0 {
			continue
		}
		basicChildren = append(basicChildren, buildChartSubgroups(label, imgs))
	}
	// Charts group: separate into Basic Charts and Addon Charts (with subgroups)
	if g := merged[listgenerator.SourceGroupCharts]; g != nil && g.Count() > 0 {
		// Basic charts: fleet, system charts, core infrastructure
		var basicChartNodes []treeNode
		// Addon charts grouped by category
		addonChartsByCategory := make(map[string][]treeNode)

		for _, name := range chartNamesSorted {
			cg := chartGroups[name]
			if cg == nil {
				continue
			}
			// Partner and UI plugin charts get their own tree roots below.
			if cg.Repo == listgenerator.SourceGroupPartnerCharts ||
				cg.Repo == listgenerator.SourceGroupUIPluginCharts {
				continue
			}
			var imgs []string
			for img := range cg.LinuxImages {
				imgs = append(imgs, img)
			}
			for img := range cg.WindowsImages {
				imgs = append(imgs, img)
			}
			sort.Strings(imgs)
			cat := ""
			if cg.Category != "" {
				cat = " [" + cg.Category + "]"
			}
			ver := inferChartVersion(imgs)
			verLabel := ""
			if ver != "" {
				verLabel = " " + ver
			}
			chartNode := treeNode{
				Id: name, Label: name + verLabel + cat,
				Kind: "chart", Count: cg.Count(),
				Version: ver, Category: cg.Category,
				Description: cg.Description,
				Children:    refsToTreeNodes(imgs),
			}
			cc.applyChartMetadata(&chartNode, name)

			// Categorize: Basic charts = ONLY auto-deployed Rancher system charts
			// Categorize: Basic charts = auto-deployed / required Rancher system charts.
			// Turtles is Essentials (2.13+ default; only CAPI path in 2.14+).
			// provisioning-capi is Essentials only for Rancher < 2.14.
			legacyEmbeddedCAPI := false
			if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.14.0-0"); ok < 0 {
				legacyEmbeddedCAPI = true
			}
			isBasic := name == "fleet" || name == "fleet-crd" || name == "fleet-agent" || name == "fleet-controller" ||
				name == "rancher-webhook" ||
				name == "rancher-turtles" ||
				name == "system-upgrade-controller" ||
				name == "remotedialer-proxy" ||
				name == "ui-plugin-operator" || name == "ui-plugin-operator-crd" ||
				name == "cert-manager" ||
				(legacyEmbeddedCAPI && name == "rancher-provisioning-capi")
			if isBasic {
				if chartNode.Category == "" || chartNode.Category == "cluster-api" {
					chartNode.Category = "system"
				}
				if chartNode.Description == "" {
					chartNode.Description = chartCategoryDescriptions["system"]
				}
				basicChartNodes = append(basicChartNodes, chartNode)
			} else {
				// Addon chart: group by category
				category := cg.Category
				if category == "" {
					// Infer category from chart name (aligned with Rancher image/chart grouping)
					nameLower := strings.ToLower(name)

					// Cloud Provider Operators (Provisioning: AKS, EKS, GKE, Ali, Azure)
					if strings.Contains(nameLower, "eks") || strings.Contains(nameLower, "gke") ||
						strings.Contains(nameLower, "aks") || strings.Contains(nameLower, "ali") ||
						strings.Contains(nameLower, "azure") && (strings.Contains(nameLower, "operator") || strings.Contains(nameLower, "service")) ||
						strings.Contains(nameLower, "provisioning") || strings.Contains(nameLower, "capi-provider") {
						category = "provisioning"
					} else if strings.Contains(nameLower, "monitoring") || strings.Contains(nameLower, "appco-") ||
						strings.Contains(nameLower, "prometheus") || strings.Contains(nameLower, "grafana") ||
						strings.Contains(nameLower, "thanos") || strings.Contains(nameLower, "alertmanager") ||
						strings.Contains(nameLower, "node-exporter") || strings.Contains(nameLower, "kube-state-metrics") ||
						strings.Contains(nameLower, "kube-rbac-proxy") || strings.Contains(nameLower, "redis") {
						category = "monitoring"
					} else if strings.Contains(nameLower, "logging") || strings.Contains(nameLower, "fluent") {
						category = "logging"
					} else if strings.Contains(nameLower, "backup") || strings.Contains(nameLower, "velero") ||
						strings.Contains(nameLower, "backup-restore-operator") {
						category = "backup-restore"
					} else if strings.Contains(nameLower, "longhorn") || strings.Contains(nameLower, "harvester") ||
						strings.Contains(nameLower, "storage") || strings.Contains(nameLower, "csi-") ||
						strings.Contains(nameLower, "local-path") {
						category = "storage"
					} else if strings.Contains(nameLower, "neuvector") || strings.Contains(nameLower, "gatekeeper") ||
						strings.Contains(nameLower, "security") || strings.Contains(nameLower, "scc") {
						category = "security"
					} else if strings.Contains(nameLower, "cis") || strings.Contains(nameLower, "compliance") ||
						strings.Contains(nameLower, "security-scan") {
						category = "cis"
					} else if strings.Contains(nameLower, "cluster-api") {
						// Upstream CAPI providers only — turtles is Essentials (isBasic above)
						category = "cluster-api"
					} else if strings.Contains(nameLower, "elemental") {
						category = "os-management"
					} else if strings.Contains(nameLower, "fleet") {
						category = "fleet"
					} else {
						category = "other"
					}
				}
				// Map unknown categories to "other"
				knownCats := map[string]bool{
					"monitoring": true, "logging": true, "backup-restore": true,
					"storage": true, "security": true, "cis": true,
					"provisioning": true, "networking": true, "cluster-api": true,
					"os-management": true, "support": true, "other": true,
				}
				if !knownCats[category] {
					category = "other"
				}
				chartNode.Category = category
				if chartNode.Description == "" {
					chartNode.Description = chartCategoryDescriptions[category]
				}
				addonChartsByCategory[category] = append(addonChartsByCategory[category], chartNode)
			}
		}

		// Build Addon Charts subgroups
		var addonSubgroups []treeNode
		categoryOrder := []string{"monitoring", "logging", "backup-restore", "storage", "security", "cis", "provisioning", "networking", "cluster-api", "os-management", "support", "other"}
		categoryNames := map[string]string{
			"monitoring":     "Monitoring",
			"logging":        "Logging",
			"backup-restore": "Backup & Restore",
			"storage":        "Storage",
			"security":       "Security",
			"cis":            "CIS Benchmark",
			"provisioning":   "Provisioning (EKS/GKE/AKS/vSphere)",
			"networking":     "Networking (Istio/SR-IOV)",
			"cluster-api":    "Cluster API",
			"os-management":  "OS Management",
			"support":        "Support & Diagnostics",
			"other":          "Other",
		}
		for _, cat := range categoryOrder {
			if charts, ok := addonChartsByCategory[cat]; ok && len(charts) > 0 {
				totalImgs := 0
				for _, ch := range charts {
					totalImgs += ch.Count
				}
				addonSubgroups = append(addonSubgroups, treeNode{
					Id: "addon_" + cat, Label: categoryNames[cat],
					Kind: "component", Count: totalImgs,
					Description: chartCategoryDescriptions[cat],
					Children:    charts,
				})
			}
		}

		// Merge Rancher system charts into the Rancher component group (avoid duplicates).
		// System charts from the repo (fleet, rancher-webhook, etc.) are matched to
		// inferred charts from KDM images. Matching charts get their images merged;
		// unmatched repo charts are added as new chart nodes under Rancher.
		if len(basicChartNodes) > 0 {
			for i, comp := range basicChildren {
				if comp.Id != "basic_rancher" {
					continue
				}
				inferredByName := make(map[string]int)
				for j, ch := range comp.Children {
					inferredByName[strings.TrimPrefix(ch.Id, "basic_chart_")] = j
				}
				for _, repoChart := range basicChartNodes {
					repoName := repoChart.Id
					if idx, ok := inferredByName[repoName]; ok {
						// Merge: add repo chart images that aren't already in the inferred chart
						existing := make(map[string]bool)
						for _, child := range comp.Children[idx].Children {
							existing[child.Id] = true
						}
						for _, child := range repoChart.Children {
							if !existing[child.Id] {
								comp.Children[idx].Children = append(comp.Children[idx].Children, child)
								comp.Children[idx].Count++
							}
						}
						comp.Children[idx].Label = repoName + " [auto-deployed]"
					} else {
						repoChart.Label = repoChart.Label + " [auto-deployed]"
						repoChart.Id = "basic_chart_" + repoName
						comp.Children = append(comp.Children, repoChart)
					}
				}
				// Recount
				total := 0
				for _, ch := range comp.Children {
					total += ch.Count
				}
				comp.Count = total
				comp.Label = "Rancher (" + strconv.Itoa(total) + " images, " + strconv.Itoa(len(comp.Children)) + " charts)"
				basicChildren[i] = comp
				break
			}
		}
		basicCharts = basicChartNodes

		// Group 2: AddOns (only addon charts with subgroups, no basic charts)
		if len(addonSubgroups) > 0 {
			totalAddonImgs := 0
			for _, sg := range addonSubgroups {
				totalAddonImgs += sg.Count
			}
			roots = append(roots, treeNode{
				Id: "addons", Label: "AddOns",
				Kind: "component", Count: totalAddonImgs,
				Description: "Optional Rancher marketplace charts — monitoring, logging, backup, storage, security, and more.",
				Children:    addonSubgroups,
			})
		}
	}

	// Opt-in chart repo roots: Partner Charts and UI Plugins (Step 1 toggles).
	if cc.includePartnerCharts {
		if node, ok := cc.buildChartRepoRoot(chartGroups, chartNamesSorted,
			listgenerator.SourceGroupPartnerCharts, "Partner Charts",
			"Rancher partner charts (rancher/partner-charts) — third-party applications validated for Rancher."); ok {
			roots = append(roots, node)
		}
	}
	if cc.includeUIPluginCharts {
		if node, ok := cc.buildChartRepoRoot(chartGroups, chartNamesSorted,
			listgenerator.SourceGroupUIPluginCharts, "UI Plugins",
			"Rancher UI plugin charts (rancher/ui-plugin-charts) — dashboard extensions."); ok {
			roots = append(roots, node)
		}
	}

	// Group 1: Essentials (appended after charts so System Charts subgroup is included)
	roots = append([]treeNode{{
		Id: "basic", Label: "Essentials",
		Kind: "component", Count: len(basicImgs),
		Description: "Required images for Rancher, your Kubernetes distro, selected CNI, and ingress/load balancer.",
		Children:    basicChildren,
	}}, roots...)

	// Group 3: Application Collection — Charts (from API refs) + Containers subgroup
	// Charts come from cc.appCollectionChartRefs (oci://dp.apps.rancher.io/charts/<slug>); generator does not extract OCI chart images, so we list chart names only.
	hasAppCollContainers := false
	var containerOnlyImgs []string
	if gCont := merged[listgenerator.SourceGroupAppCollectionContainers]; gCont != nil && gCont.Count() > 0 {
		hasAppCollContainers = true
		for img := range gCont.LinuxImages {
			containerOnlyImgs = append(containerOnlyImgs, img)
		}
		for img := range gCont.WindowsImages {
			if !gCont.LinuxImages[img] {
				containerOnlyImgs = append(containerOnlyImgs, img)
			}
		}
		sort.Strings(containerOnlyImgs)
	}
	hasAppCollCharts := len(cc.appCollectionChartRefs) > 0
	if hasAppCollCharts || hasAppCollContainers {
		// Metadata (display name, description, logo, category) by chart/image ref.
		appByChartRef := make(map[string]appcollection.Application, len(cc.appCollectionApps))
		appByImageRef := make(map[string]appcollection.Application, len(cc.appCollectionApps))
		for _, app := range cc.appCollectionApps {
			if app.ChartRef != "" {
				appByChartRef[app.ChartRef] = app
			}
			if app.ImageRef != "" {
				appByImageRef[app.ImageRef] = app
			}
		}
		var appCollChartNodes []treeNode
		for _, ref := range cc.appCollectionChartRefs {
			// ref is oci://dp.apps.rancher.io/charts/<slug>
			slug := ref
			if idx := strings.LastIndex(ref, "/"); idx >= 0 {
				slug = ref[idx+1:]
			}
			slug = strings.TrimPrefix(slug, "charts/")
			label := slug
			if label == "" {
				label = ref
			}
			node := treeNode{
				Id: ref, Label: label,
				Kind: "chart", Count: 0, Children: nil,
			}
			if app, ok := appByChartRef[ref]; ok {
				if app.Name != "" {
					node.Label = app.Name
				}
				node.Description = app.Description
				node.Category = app.Category
				node.IconURL = app.LogoURL
			}
			appCollChartNodes = append(appCollChartNodes, node)
		}
		sort.Slice(appCollChartNodes, func(i, j int) bool { return appCollChartNodes[i].Label < appCollChartNodes[j].Label })
		totalCharts := len(appCollChartNodes)
		var appCollChildren []treeNode
		if totalCharts > 0 {
			appCollChildren = append(appCollChildren, treeNode{
				Id: "app_collection_charts", Label: "Helm Charts (" + strconv.Itoa(totalCharts) + " OCI refs)",
				Kind: "component", Count: totalCharts, Children: appCollChartNodes,
			})
		}
		if len(containerOnlyImgs) > 0 {
			containerNodes := refsToTreeNodes(containerOnlyImgs)
			for i := range containerNodes {
				if app, ok := appByImageRef[containerNodes[i].Id]; ok {
					containerNodes[i].Description = app.Description
					containerNodes[i].Category = app.Category
					containerNodes[i].IconURL = app.LogoURL
				}
			}
			appCollChildren = append(appCollChildren, treeNode{
				Id: listgenerator.SourceGroupAppCollectionContainers, Label: "Container Images (" + strconv.Itoa(len(containerOnlyImgs)) + ")",
				Kind: "component", Count: len(containerOnlyImgs), Children: containerNodes,
			})
		}
		totalAppColl := totalCharts + len(containerOnlyImgs)
		roots = append(roots, treeNode{
			Id: "app_collection", Label: "Application Collection (" + strconv.Itoa(totalAppColl) + ")",
			Kind: "component", Count: totalAppColl, Children: appCollChildren,
		})
	}

	// Collect ALL charts that have images in Basic group (not just Fleet and CNI)
	// This includes Fleet, CNI, Rancher component charts (like rancher-webhook, rancher-turtles), etc.
	basicImageSet := make(map[string]bool)
	for _, img := range basicImgs {
		basicImageSet[img] = true
	}

	var basicChartsForPreview []treeNode
	seenBasicCharts := make(map[string]bool)

	// Addon chart categories: these must not appear under Basic (only under AddOns).
	addonChartCategories := map[string]bool{
		"monitoring": true, "logging": true, "backup-restore": true,
		"storage": true, "security": true, "cis": true,
		"provisioning": true, "cluster-api": true, "os-management": true, "other": true,
	}

	// Find all charts that have at least one image in Basic and are actually basic charts
	// (exclude addon charts like rancher-monitoring, rancher-monitoring-crd, rancher-logging, etc.)
	for name, cg := range chartGroups {
		if cg == nil {
			continue
		}
		if addonChartCategories[cg.Category] {
			continue
		}
		// Check if this chart has any images in Basic
		hasBasicImage := false
		for img := range cg.LinuxImages {
			if basicImageSet[img] {
				hasBasicImage = true
				break
			}
		}
		if !hasBasicImage {
			for img := range cg.WindowsImages {
				if basicImageSet[img] {
					hasBasicImage = true
					break
				}
			}
		}

		if hasBasicImage && !seenBasicCharts[name] {
			seenBasicCharts[name] = true
			var imgs []string
			for img := range cg.LinuxImages {
				if basicImageSet[img] {
					imgs = append(imgs, img)
				}
			}
			for img := range cg.WindowsImages {
				if basicImageSet[img] {
					imgs = append(imgs, img)
				}
			}
			sort.Strings(imgs)
			cat := ""
			if cg.Category != "" {
				cat = " [" + cg.Category + "]"
			}
			basicChartsForPreview = append(basicChartsForPreview, treeNode{
				Id: name, Label: name + cat,
				Kind: "chart", Count: len(imgs), Children: refsToTreeNodes(imgs),
			})
		}
	}

	// Always include main Rancher chart(s) in Basic charts preview (may have no/small overlap in chartGroups)
	for _, name := range []string{"rancher-rancher", "rancher"} {
		if seenBasicCharts[name] {
			continue
		}
		seenBasicCharts[name] = true
		var imgs []string
		if cg := chartGroups[name]; cg != nil {
			for img := range cg.LinuxImages {
				if basicImageSet[img] {
					imgs = append(imgs, img)
				}
			}
			for img := range cg.WindowsImages {
				if basicImageSet[img] {
					imgs = append(imgs, img)
				}
			}
		}
		if len(imgs) == 0 {
			// Well-known rancher/rancher image(s) — one per Rancher version when multiple selected
			if len(cc.rancherVersionsList) > 0 {
				for _, rt := range cc.rancherVersionsList {
					if rt == "" {
						rt = "latest"
					}
					imgs = append(imgs, "rancher/rancher:"+rt)
				}
			} else {
				rt := cc.rancherVersion
				if rt == "" {
					rt = "latest"
				}
				imgs = append(imgs, "rancher/rancher:"+rt)
			}
		}
		sort.Strings(imgs)
		label := name + " [core]"
		basicChartsForPreview = append(basicChartsForPreview, treeNode{
			Id: name, Label: label,
			Kind: "chart", Count: len(imgs), Children: refsToTreeNodes(imgs),
		})
	}

	// Keep separate lists for backward compatibility (though Basic charts now includes all)
	var fleetChartsForPreview []treeNode
	var cniChartsForPreview []treeNode
	for _, chart := range basicChartsForPreview {
		if strings.Contains(chart.Label, "fleet") || strings.Contains(chart.Id, "fleet") {
			fleetChartsForPreview = append(fleetChartsForPreview, chart)
		}
		if strings.Contains(chart.Label, "calico") || strings.Contains(chart.Label, "flannel") ||
			strings.Contains(chart.Label, "canal") || strings.Contains(chart.Label, "cni") ||
			strings.Contains(chart.Id, "calico") || strings.Contains(chart.Id, "flannel") ||
			strings.Contains(chart.Id, "canal") || strings.Contains(chart.Id, "cni") {
			cniChartsForPreview = append(cniChartsForPreview, chart)
		}
	}

	// Build past-selection summary for Step 3 footer: Rancher version, Step 1 (source), Step 2 (distro, CNI, LB, K8s versions)
	distros := strings.Split(cc.components, ",")
	for i, d := range distros {
		distros[i] = strings.TrimSpace(d)
	}
	rancherVer := cc.rancherVersion
	if rancherVer == "" {
		rancherVer = "latest"
	}
	sourceStr := "Community"
	if cc.isRPMGC {
		sourceStr = "Rancher Prime"
	}
	step1Str := rancherVer + " · " + sourceStr
	pastStep2 := formatStep1PastDistro(distros, cc.interactiveIncludeWindows) +
		"; CNI: " + formatStep1PastCNI(cc.interactiveSelectedCNI) +
		"; LB: " + formatStep1PastLB(LBOptions{
		K3sKlipper:  cc.interactiveLBK3sKlipper,
		K3sTraefik:  cc.interactiveLBK3sTraefik,
		RKE2Nginx:   cc.interactiveLBRKE2Nginx,
		RKE2Traefik: cc.interactiveLBRKE2Traefik,
	})
	// Add selected Kubernetes versions (only for distros that were actually selected)
	compParts := strings.Split(cc.components, ",")
	compSet := make(map[string]bool)
	for _, c := range compParts {
		compSet[strings.TrimSpace(c)] = true
	}
	var versParts []string
	if compSet["k3s"] && cc.k3sVersions != "" {
		v := cc.k3sVersions
		if len(v) > 20 {
			v = v[:17] + "..."
		}
		versParts = append(versParts, "K3s: "+v)
	}
	if compSet["rke2"] && cc.rke2Versions != "" {
		v := cc.rke2Versions
		if len(v) > 20 {
			v = v[:17] + "..."
		}
		versParts = append(versParts, "RKE2: "+v)
	}
	if compSet["rke"] && cc.rkeVersions != "" {
		v := cc.rkeVersions
		if len(v) > 20 {
			v = v[:17] + "..."
		}
		versParts = append(versParts, "RKE1: "+v)
	}
	if len(versParts) > 0 {
		pastStep2 += "; " + strings.Join(versParts, " ")
	}
	arch := cc.targetArch
	if arch == "" {
		arch = "amd64"
	}
	pastStep2 += "; arch: linux/" + arch
	pastSelection = step1Str + "  →  " + pastStep2
	return roots, basicChartsForPreview, fleetChartsForPreview, cniChartsForPreview, basicImageComponent, pastSelection
}

// runInteractiveTUI runs the tree TUI: 3 presets + 4 groups; each expandable to show images.
func (cc *genesisCmd) runInteractiveTUI() error {
	roots, basicChartsForPreview, fleetChartsForPreview, cniChartsForPreview, basicImageComponent, pastSelection := cc.buildGenesisTree()
	componentIDs, chartNames, selectedImageRefs, err := runTreeTUI(roots, cc.interactiveSelectedCNI, cc.components, basicChartsForPreview, fleetChartsForPreview, cniChartsForPreview, basicImageComponent, pastSelection)
	if err != nil {
		return err
	}
	cc.interactiveSelectedComponentIDs = componentIDs
	cc.interactiveSelectedChartNames = chartNames
	cc.interactiveSelectedImageRefs = selectedImageRefs

	// Print summary and continue to Step 3 (finish)
	fmt.Println("\n=== Step 3 complete ===")
	if len(componentIDs) > 0 {
		fmt.Printf("Selected components: %s\n", strings.Join(componentIDs, ", "))
	}
	if len(chartNames) > 0 {
		fmt.Printf("Selected charts: %d\n", len(chartNames))
	}
	fmt.Println("\n=== Step 4: Generating image list ===")
	return nil
}

func imageRefsFromMaps(linux, windows map[string]map[string]bool) []string {
	var out []string
	for img := range linux {
		out = append(out, img)
	}
	for img := range windows {
		out = append(out, img)
	}
	sort.Strings(out)
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// excludeOptionalStorageFromEssentials reports whether an image should be kept
// out of the Essentials/Basic preset as optional storage or infra cloud-provider
// tooling. Distro defaults required to create clusters must NOT match:
//   - rke2-cloud-provider (RKE2 CCM from rke2-images-core)
//   - local-path-provisioner (default K3s storage from k3s-images.txt)
func excludeOptionalStorageFromEssentials(imgLower string) bool {
	// Keep RKE2 / K3s defaults that are required for cluster create.
	if strings.Contains(imgLower, "rke2-cloud-provider") ||
		strings.Contains(imgLower, "local-path-provisioner") {
		return false
	}
	// Optional marketplace / infra storage & cloud providers.
	if strings.Contains(imgLower, "harvester") ||
		strings.Contains(imgLower, "longhorn") ||
		strings.Contains(imgLower, "vsphere") ||
		strings.Contains(imgLower, "cloud-provider") {
		return true
	}
	// Generic CSI sidecars / out-of-tree drivers — not CNI-bundled Calico CSI
	// (mirrored-calico-csi has no "csi-" substring).
	if strings.Contains(imgLower, "csi-") && !strings.Contains(imgLower, "calico") {
		return true
	}
	return false
}

// filterImagesByVersions filters images based on selected Kubernetes versions.
// For K3s: matches images like rancher/k3s-upgrade:v1.28.15-k3s1
// For RKE2: matches images like rancher/rke2-upgrade:v1.28.15-rke2r1
// For RKE1: images don't have version tags, so we include all if RKE1 is selected
func filterImagesByVersions(images []string, k3sVers, rke2Vers, rkeVers string) []string {
	if k3sVers == "all" && rke2Vers == "all" && rkeVers == "all" {
		return images // No filtering needed
	}

	// Parse version lists
	k3sVersMap := make(map[string]bool)
	if k3sVers != "" && k3sVers != "all" {
		for _, v := range strings.Split(k3sVers, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				k3sVersMap[v] = true
			}
		}
	}
	rke2VersMap := make(map[string]bool)
	if rke2Vers != "" && rke2Vers != "all" {
		for _, v := range strings.Split(rke2Vers, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				rke2VersMap[v] = true
			}
		}
	}
	rkeVersMap := make(map[string]bool)
	if rkeVers != "" && rkeVers != "all" {
		for _, v := range strings.Split(rkeVers, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				rkeVersMap[v] = true
			}
		}
	}

	var filtered []string
	for _, img := range images {
		include := false

		// Check if it's a K3s image (k3s-upgrade, system-agent-installer-k3s, or from k3s-images.txt)
		if strings.Contains(img, "k3s-upgrade:") || strings.Contains(img, "system-agent-installer-k3s:") {
			if len(k3sVersMap) == 0 {
				include = true // "all" selected
			} else {
				// Extract version from tag like v1.28.15-k3s1
				parts := strings.Split(img, ":")
				if len(parts) == 2 {
					tag := parts[1]
					// Remove -k3s1, -k3s2 suffix
					tag = strings.TrimSuffix(tag, "-k3s1")
					tag = strings.TrimSuffix(tag, "-k3s2")
					tag = strings.TrimSuffix(tag, "-k3s3")
					// Check if this version matches
					if k3sVersMap[tag] {
						include = true
					}
				}
			}
		} else if strings.Contains(img, "rke2-upgrade:") || strings.Contains(img, "system-agent-installer-rke2:") {
			// RKE2 image
			if len(rke2VersMap) == 0 {
				include = true // "all" selected
			} else {
				parts := strings.Split(img, ":")
				if len(parts) == 2 {
					tag := parts[1]
					// Remove -rke2r1, -rke2r2 suffix
					for strings.Contains(tag, "-rke2r") {
						idx := strings.LastIndex(tag, "-rke2r")
						if idx > 0 {
							tag = tag[:idx]
							break
						}
						break
					}
					if rke2VersMap[tag] {
						include = true
					}
				}
			}
		} else if strings.Contains(img, "rke") && !strings.Contains(img, "rke2") {
			// RKE1 image (no version tags, include if RKE1 is selected)
			if len(rkeVersMap) == 0 {
				include = true // "all" selected or RKE1 not filtered
			} else {
				// RKE1 images don't have version tags, so we include all if any RKE1 version is selected
				include = true
			}
		} else {
			// Chart images or other images from external lists (k3s-images.txt, rke2-images-all.txt)
			// These don't have version tags, but if versions are filtered, the generator should
			// have already filtered them at the source level. Include them here.
			// If specific versions are selected, we can't filter these by tag, so include all.
			// The generator's version filtering should have already handled this.
			include = true
		}

		if include {
			filtered = append(filtered, img)
		}
	}
	return filtered
}

func (cc *genesisCmd) parseComponentFlags() error {
	// Parse components flag
	if cc.components != "" {
		// Components are already parsed from interactive or flag
		// This will be used in prepareGenerator
	}
	// Versions are already set from interactive or flags
	// Charts selection is already set
	return nil
}

func (cc *genesisCmd) prepareGenerator() error {
	option := &listgenerator.GeneratorOption{
		RancherVersion: cc.rancherVersion,
		MinKubeVersion: "",
		LinuxArch:      cc.targetArch,
		ChartsPaths:    make(map[string]chartimages.ChartRepoType),
		ChartURLs: make(map[string]struct {
			Type   chartimages.ChartRepoType
			Branch string
		}),
		InsecureSkipTLS:     !cc.tlsVerify,
		RemoveDeprecatedKDM: cc.kdmRemoveDeprecated,
	}

	if cc.minKubeVersion != "" {
		minKubeVersion := semver.MajorMinor(cc.minKubeVersion)
		option.MinKubeVersion = minKubeVersion
		if minKubeVersion == "" {
			return fmt.Errorf("invalid min-kube-version provided: %v",
				cc.minKubeVersion)
		}
	} else {
		switch {
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.7"):
			option.MinKubeVersion = "v1.23.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.8"):
			option.MinKubeVersion = "v1.25.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.9"):
			option.MinKubeVersion = "v1.27.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.10"):
			option.MinKubeVersion = "v1.28.0"
		case utils.SemverMajorMinorEqual(cc.rancherVersion, "v2.11"):
			option.MinKubeVersion = "v1.30.0"
		// v2.12+ does not support RKE1
		default:
			option.MinKubeVersion = "v1.30.0"
		}
	}
	if cc.kdm != "" {
		if _, err := url.ParseRequestURI(cc.kdm); err != nil {
			option.KDMPath = cc.kdm
		} else {
			option.KDMURL = cc.kdm
		}
	}

	charts := cc.charts
	if len(charts) != 0 {
		for _, chart := range charts {
			if _, err := url.ParseRequestURI(chart); err != nil {
				logrus.Debugf("Add chart path to load images: %q", chart)
				option.ChartsPaths[chart] = chartimages.RepoTypeDefault
			} else {
				// cc.generator.ChartURLs[chart] = struct {
				// 	Type   chartimages.ChartRepoType
				// 	Branch string
				// }{
				// 	Type:   chartimages.RepoTypeDefault,
				// 	Branch: "", // use default branch
				// }
				return fmt.Errorf("chart url is not supported, please provide the cloned chart path")
			}
		}
	}
	systemCharts := cc.systemCharts
	if len(systemCharts) != 0 {
		for _, chart := range systemCharts {
			if _, err := url.ParseRequestURI(chart); err != nil {
				logrus.Debugf("Add system chart path to load images: %q", chart)
				option.ChartsPaths[chart] = chartimages.RepoTypeSystem
			} else {
				return fmt.Errorf("chart url is not supported, please provide the cloned chart path")
			}
		}
	}
	dev := cc.dev
	if cc.kdm == "" && len(charts) == 0 && len(systemCharts) == 0 {
		if cc.isRPMGC || cc.includeCommunityImageLists {
			if dev {
				logrus.Info("Using branch: dev")
			} else {
				logrus.Info("Using branch: release")
			}
			addRancherPrimeCharts(cc.rancherVersion, option, dev)
			addRancherPrimeSystemCharts(cc.rancherVersion, option, dev)
			addRancherPrimeKontainerDriverMetadata(cc.rancherVersion, option, dev)
		}
		usePrimeLists := cc.isRPMGC
		useCommunityLists := cc.includeCommunityImageLists
		if !usePrimeLists && !useCommunityLists {
			useCommunityLists = true
		}
		if usePrimeLists {
			logrus.Debugf("Image lists: Rancher Prime Registry (%s)", PrimeImageListBaseURL)
			option.ImageListBaseURL = PrimeImageListBaseURL
		}
		if useCommunityLists {
			logrus.Debugf("Image lists: Community GitHub releases (k3s/rke2/rancher-images.txt)")
			// When Prime is also enabled, K3s/RKE2 still come from prime.ribs; this adds
			// rancher-images.txt from GitHub (with prime.ribs fallback on 404).
			option.RancherImagesTxtURL = GetRancherImagesTxtURL(cc.rancherVersion)
		}
	}

	// Opt-in extra chart repos (Step 1 toggles / flags)
	if cc.includePartnerCharts {
		logrus.Debugf("Add Rancher partner charts (rancher/partner-charts)")
		addRancherPartnerCharts(option)
	}
	if cc.includeUIPluginCharts {
		logrus.Debugf("Add Rancher UI plugin charts (rancher/ui-plugin-charts)")
		addRancherUIPluginCharts(option)
	}

	// Live-fetch Application Collection (charts + container images) when enabled
	if cc.includeAppCollectionCharts {
		user := cc.appCollectionAPIUser
		pass := cc.appCollectionAPIPassword
		if user == "" {
			user = os.Getenv("RANCHER_APPS_API_USER")
		}
		if pass == "" {
			pass = os.Getenv("RANCHER_APPS_API_PASSWORD")
		}
		apps, err := appcollection.FetchApplicationsMetadata(signalContext, user, pass)
		if err != nil {
			return fmt.Errorf("fetch Application Collection: %w (set RANCHER_APPS_API_USER and RANCHER_APPS_API_PASSWORD for auth)", err)
		}
		var chartRefs, imageRefs []string
		for _, app := range apps {
			switch app.Type {
			case appcollection.TypeHelmChart:
				chartRefs = append(chartRefs, app.ChartRef)
			case appcollection.TypeSingleContainer:
				imageRefs = append(imageRefs, app.ImageRef)
			}
		}
		option.AppCollectionCharts = chartRefs
		option.AppCollectionImages = imageRefs
		cc.appCollectionChartRefs = chartRefs
		cc.appCollectionApps = apps
		logrus.Infof("Application Collection: %d charts, %d container images (helm registry login %s for OCI chart pull)", len(chartRefs), len(imageRefs), appcollection.ChartsRegistry)
	}

	// Apply component selection filters
	if err := cc.applyComponentFilters(option); err != nil {
		return err
	}
	// Log Min RKE1 Version only when RKE1 is actually included in this run
	if n, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); n < 0 {
		includeRKE1 := len(option.IncludeClusterTypes) == 0
		if !includeRKE1 {
			for _, t := range option.IncludeClusterTypes {
				if t == kdmimages.RKE {
					includeRKE1 = true
					break
				}
			}
		}
		if includeRKE1 {
			logrus.Infof("Min RKE1 Version for Rancher [%v]: %v", cc.rancherVersion, option.MinKubeVersion)
		}
	}
	option.OnProgress = GenesisProgressCallback
	g, err := listgenerator.NewGenerator(option)
	if err != nil {
		return err
	}
	cc.generator = g

	return nil
}

func (cc *genesisCmd) applyComponentFilters(option *listgenerator.GeneratorOption) error {
	// Parse cluster types
	if cc.components != "" {
		parts := strings.Split(cc.components, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			switch part {
			case "1", "k3s":
				option.IncludeClusterTypes = append(option.IncludeClusterTypes, kdmimages.K3S)
			case "2", "rke2":
				option.IncludeClusterTypes = append(option.IncludeClusterTypes, kdmimages.RKE2)
			case "3", "rke":
				if ok, _ := utils.SemverCompare(cc.rancherVersion, "v2.12.0-0"); ok < 0 {
					option.IncludeClusterTypes = append(option.IncludeClusterTypes, kdmimages.RKE)
				}
			case "charts":
				// Charts are handled separately
			}
		}
	}

	// Parse version filters
	if cc.k3sVersions != "" && cc.k3sVersions != "all" {
		versions := strings.Split(cc.k3sVersions, ",")
		for _, v := range versions {
			v = strings.TrimSpace(v)
			if v != "" {
				option.IncludeK3sVersions = append(option.IncludeK3sVersions, v)
			}
		}
	}
	if cc.rke2Versions != "" && cc.rke2Versions != "all" {
		versions := strings.Split(cc.rke2Versions, ",")
		for _, v := range versions {
			v = strings.TrimSpace(v)
			if v != "" {
				option.IncludeRKE2Versions = append(option.IncludeRKE2Versions, v)
			}
		}
	}
	if cc.rkeVersions != "" && cc.rkeVersions != "all" {
		versions := strings.Split(cc.rkeVersions, ",")
		for _, v := range versions {
			v = strings.TrimSpace(v)
			if v != "" {
				option.IncludeRKE1Versions = append(option.IncludeRKE1Versions, v)
			}
		}
	}

	// Parse chart selection
	if cc.chartsSelection == "none" {
		option.IncludeChartImages = false
	} else if cc.chartsSelection == "all" || cc.chartsSelection == "" {
		option.IncludeChartImages = true
	} else {
		// Specific chart names
		option.IncludeChartImages = true
		chartNames := strings.Split(cc.chartsSelection, ",")
		for _, name := range chartNames {
			name = strings.TrimSpace(name)
			if name != "" {
				option.IncludeChartNames = append(option.IncludeChartNames, name)
			}
		}
	}

	// Default: if no components specified and charts not explicitly set to "none",
	// include everything (current behavior)
	if len(option.IncludeClusterTypes) == 0 && cc.components == "" {
		// No filtering - include all cluster types
	}
	if !option.IncludeChartImages && cc.chartsSelection == "" {
		// Default to including charts if not explicitly disabled
		option.IncludeChartImages = true
	}

	return nil
}

func (cc *genesisCmd) run(ctx context.Context) error {
	err := cc.generator.Run(ctx)
	if err == nil && cc.includeCertManager && cc.generator != nil {
		listgenerator.MergeCertManagerImages(cc.generator.LinuxImages, listgenerator.DefaultCertManagerVersion)
		logrus.Infof("Included cert-manager %s images (Rancher Helm install prerequisite)", listgenerator.DefaultCertManagerVersion)
	}

	// Cleanup cache (if exists) after generate image list. Serve mode keeps
	// the cache during the request, then handleGenerate cleans up on success.
	if !cc.keepChartCache {
		cleanupChartCloneCache()
	}
	return err
}

func cleanupChartCloneCache() {
	cacheDir := filepath.Join(utils.HangarCacheDir(), utils.CacheCloneRepoDirectory)
	if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
		logrus.Warnf("Failed to delete %q: %v", cacheDir, err)
	}
}

func (cc *genesisCmd) finish() error {
	totalLinux := len(cc.generator.LinuxImages)
	totalWindows := len(cc.generator.WindowsImages)
	// Apply selection filtering whenever a selection was made, regardless of
	// interactive mode. Config-file mode and the API export path set these
	// selections but run with interactive=false; without this the exported
	// list would ignore the user's group/chart selection and dump every image.
	// imageRefs alone (e.g. frontend preview export) must also trigger filtering.
	if len(cc.interactiveSelectedComponentIDs) > 0 || len(cc.interactiveSelectedChartNames) > 0 || len(cc.interactiveSelectedImageRefs) > 0 {
		if len(cc.interactiveSelectedImageRefs) > 0 {
			// Use exact image set from TUI so output matches preview
			refSet := make(map[string]bool)
			for _, ref := range cc.interactiveSelectedImageRefs {
				refSet[ref] = true
			}
			linuxFiltered := make(map[string]map[string]bool)
			windowsFiltered := make(map[string]map[string]bool)
			for img, sources := range cc.generator.LinuxImages {
				if refSet[img] {
					linuxFiltered[img] = sources
				}
			}
			for img, sources := range cc.generator.WindowsImages {
				if refSet[img] {
					windowsFiltered[img] = sources
				}
			}
			// Include refs from TUI that are not in the generator (e.g. rancher/rancher - main server)
			// so they appear in the output list
			for _, ref := range cc.interactiveSelectedImageRefs {
				if linuxFiltered[ref] == nil && windowsFiltered[ref] == nil {
					linuxFiltered[ref] = map[string]bool{"[basic]": true}
				}
			}
			cc.generator.LinuxImages = linuxFiltered
			cc.generator.WindowsImages = windowsFiltered
		} else {
			linuxFiltered, windowsFiltered := listgenerator.FilterImageSetsBySelection(
				cc.generator.LinuxImages, cc.generator.WindowsImages,
				cc.interactiveSelectedComponentIDs, cc.interactiveSelectedChartNames)
			// Respect load balancer choices from config/TUI
			dropLB := func(img string) bool {
				imgLower := strings.ToLower(img)
				if strings.Contains(imgLower, "klipper-helm") || strings.Contains(imgLower, "klipper-lb") {
					return !cc.interactiveIncludeLB || !cc.interactiveLBK3sKlipper
				}
				if strings.Contains(imgLower, "nginx-ingress") || strings.Contains(imgLower, "ingress-nginx") || strings.Contains(imgLower, "mirrored-ingress-nginx") {
					return !cc.interactiveIncludeLB || !cc.interactiveLBRKE2Nginx
				}
				if strings.Contains(imgLower, "traefik") {
					return !cc.interactiveIncludeLB || (!cc.interactiveLBK3sTraefik && !cc.interactiveLBRKE2Traefik)
				}
				return false
			}
			{
				for img := range linuxFiltered {
					if dropLB(img) {
						delete(linuxFiltered, img)
					}
				}
				for img := range windowsFiltered {
					if dropLB(img) {
						delete(windowsFiltered, img)
					}
				}
			}
			cc.generator.LinuxImages = linuxFiltered
			cc.generator.WindowsImages = windowsFiltered
		}
	}
	// If user chose Linux only in Step 1 (distro dialog) or config, exclude all Windows images
	if !cc.interactiveIncludeWindows {
		cc.generator.WindowsImages = make(map[string]map[string]bool)
		cc.generator.RKE2WindowsImages = make(map[string]map[string]bool)
	}

	var (
		imagesLinuxList   = make([]string, 0)
		imagesWindowsList = make([]string, 0)
		imageSourcesList  = make([]string, 0)

		rke1LinuxImageList   = make([]string, 0)
		rke2LinuxImageList   = make([]string, 0)
		rke2WindowsImageList = make([]string, 0)
		k3sLinuxImageList    = make([]string, 0)

		rkeVersions  = make([]string, 0)
		rke2Versions = make([]string, 0)
		k3sVersions  = make([]string, 0)
	)

	// NOTE: Do NOT rewrite rancher/rancher-webhook → cnrancher/rancher-webhook.
	// That Hangar generate-list behavior is for China Prime Manager GC only.
	// Genesis "Rancher Prime" is international (prime.ribs / registry.rancher.com).
	for img := range cc.generator.LinuxImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		imagesLinuxList = append(imagesLinuxList, imgWithRegistry)
		imageSourcesList = append(imageSourcesList,
			fmt.Sprintf("%s %s", imgWithRegistry,
				genesisGetSourcesList(cc.generator.LinuxImages[img])))
	}
	for img := range cc.generator.WindowsImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		imagesWindowsList = append(imagesWindowsList, imgWithRegistry)
		imageSourcesList = append(imageSourcesList,
			fmt.Sprintf("%s %s", imgWithRegistry,
				genesisGetSourcesList(cc.generator.WindowsImages[img])))
	}
	for img := range cc.generator.RKE1LinuxImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		rke1LinuxImageList = append(rke1LinuxImageList, imgWithRegistry)
	}
	for img := range cc.generator.RKE2LinuxImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		rke2LinuxImageList = append(rke2LinuxImageList, imgWithRegistry)
	}
	for img := range cc.generator.RKE2WindowsImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		rke2WindowsImageList = append(rke2WindowsImageList, imgWithRegistry)
	}
	for img := range cc.generator.K3sLinuxImages {
		imgWithRegistry := img
		if cc.registry != "" {
			imgWithRegistry = utils.ConstructRegistry(img, cc.registry)
		}
		k3sLinuxImageList = append(k3sLinuxImageList, imgWithRegistry)
	}
	for v := range cc.generator.RKE1Versions {
		rkeVersions = append(rkeVersions, v)
	}
	for v := range cc.generator.RKE2Versions {
		rke2Versions = append(rke2Versions, v)
	}
	for v := range cc.generator.K3sVersions {
		k3sVersions = append(k3sVersions, v)
	}
	sort.Strings(imagesLinuxList)
	sort.Strings(imagesWindowsList)
	sort.Strings(imageSourcesList)
	sort.Strings(rke1LinuxImageList)
	sort.Strings(rke2LinuxImageList)
	sort.Strings(rke2WindowsImageList)
	sort.Strings(k3sLinuxImageList)

	sort.Slice(rkeVersions, func(i, j int) bool {
		ok, _ := utils.SemverCompare(rkeVersions[i], rkeVersions[j])
		return ok > 0
	})
	sort.Slice(rke2Versions, func(i, j int) bool {
		ok, _ := utils.SemverCompare(rke2Versions[i], rke2Versions[j])
		return ok > 0
	})
	sort.Slice(k3sVersions, func(i, j int) bool {
		ok, _ := utils.SemverCompare(k3sVersions[i], k3sVersions[j])
		return ok > 0
	})

	var scanSummaryByImage map[string]string
	if cc.scan && cc.output != "" && len(imagesLinuxList) > 0 {
		logrus.Info("Running vulnerability scan on image list (this may take a while)...")
		report, err := cc.runScanForGenerateList(signalContext, imagesLinuxList)
		if err != nil {
			logrus.Warnf("Scan failed: %v (output will not include scan annotations)", err)
		} else {
			scanSummaryByImage = genesisSummaryByImage(report)
			totals := genesisScanTotalCounts(report)
			var parts []string
			for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
				if n := totals[sev]; n > 0 {
					parts = append(parts, fmt.Sprintf("%s=%d", sev, n))
				}
			}
			if len(parts) > 0 {
				logrus.Infof("Scan complete. Vulnerabilities: %s", strings.Join(parts, " "))
			} else {
				logrus.Info("Scan complete. No vulnerabilities found.")
			}
			if cc.interactive {
				if len(parts) > 0 {
					fmt.Printf("Scan: %s (see output file for per-image).\n", strings.Join(parts, " "))
				} else {
					fmt.Println("Scan: no vulnerabilities (see output file for per-image).")
				}
			}
			scanReportPath := cc.scanReport
			if scanReportPath == "" {
				base := strings.TrimSuffix(cc.output, filepath.Ext(cc.output))
				scanReportPath = base + "-scan-report.csv"
			}
			if err := cc.saveScanReport(signalContext, report, scanReportPath); err != nil {
				logrus.Warnf("Failed to write scan report %q: %v", scanReportPath, err)
			} else {
				logrus.Infof("Scan report written to %q", scanReportPath)
			}
		}
	}

	if cc.output != "" {
		if scanSummaryByImage != nil {
			lines := make([]string, 0, len(imagesLinuxList))
			for _, img := range imagesLinuxList {
				if s, ok := scanSummaryByImage[img]; ok {
					lines = append(lines, img+" # scan: "+s)
				} else {
					lines = append(lines, img+" # scan: (scan failed or skipped)")
				}
			}
			if err := cc.saveSlice(signalContext, cc.output, lines); err != nil {
				return fmt.Errorf("failed to write file %q: %w", cc.output, err)
			}
		} else {
			if err := cc.saveSlice(signalContext, cc.output, imagesLinuxList); err != nil {
				return fmt.Errorf("failed to write file %q: %w", cc.output, err)
			}
		}
		logrus.Infof("Exported Rancher linux images into %v", cc.output)
	}
	if cc.outputWindows != "" {
		err := cc.saveSlice(signalContext, cc.outputWindows, imagesWindowsList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.outputWindows, err)
		}
		logrus.Infof("Exported Rancher windows images into %v", cc.outputWindows)
	}
	if cc.outputSource != "" {
		err := cc.saveSlice(signalContext, cc.outputSource, imageSourcesList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.outputSource, err)
		}
		logrus.Infof("Exported Rancher images sources into %v", cc.outputSource)
	}
	if cc.outputVersions != "" {
		var versions []string
		if len(rkeVersions) > 0 {
			versions = append(versions, fmt.Sprintf("K3s, RKE2, RKE versions for Rancher %v:", cc.rancherVersion))
		} else {
			versions = append(versions, fmt.Sprintf("K3s, RKE2 versions for Rancher %v:", cc.rancherVersion))
		}
		versions = append(versions, "")
		versions = append(versions, "K3s Versions:")
		versions = append(versions, k3sVersions...)
		versions = append(versions, "")
		versions = append(versions, "RKE2 Versions:")
		versions = append(versions, rke2Versions...)
		if len(rkeVersions) > 0 {
			versions = append(versions, "")
			versions = append(versions, "RKE Versions:")
			versions = append(versions, rkeVersions...)
		}
		err := cc.saveSlice(signalContext, cc.outputVersions, versions)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.outputVersions, err)
		}
		logrus.Infof("Exported Rancher supported versions into %v", cc.outputVersions)
	}
	if cc.rke1Images != "" {
		err := cc.saveSlice(signalContext, cc.rke1Images, rke1LinuxImageList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.rke1Images, err)
		}
		logrus.Infof("Exported RKE1 Linux images into %v", cc.rke1Images)
	}
	if cc.k3sImages != "" {
		err := cc.saveSlice(signalContext, cc.k3sImages, k3sLinuxImageList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.k3sImages, err)
		}
		logrus.Infof("Exported K3s Linux images into %v", cc.k3sImages)
	}
	if cc.rke2Images != "" {
		err := cc.saveSlice(signalContext, cc.rke2Images, rke2LinuxImageList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.rke2Images, err)
		}
		logrus.Infof("Exported RKE2 Linux images into %v", cc.rke2Images)
	}
	if cc.rke2WindowsImages != "" {
		err := cc.saveSlice(signalContext, cc.rke2WindowsImages, rke2WindowsImageList)
		if err != nil {
			return fmt.Errorf("failed to write file %q: %w", cc.rke2WindowsImages, err)
		}
		logrus.Infof("Exported RKE2 Windows images into %v", cc.rke2WindowsImages)
	}
	if cc.interactive {
		selectedLinux := len(cc.generator.LinuxImages)
		selectedWindows := len(cc.generator.WindowsImages)
		total := totalLinux + totalWindows
		selected := selectedLinux + selectedWindows
		fmt.Printf("\nYou selected %d of %d images from this Rancher image set (linux: %d/%d, windows: %d/%d).\n",
			selected, total, selectedLinux, totalLinux, selectedWindows, totalWindows)
	}
	return nil
}

func genesisGetSourcesList(imageSources map[string]bool) string {
	var sources = []string{}
	for source := range imageSources {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return strings.Join(sources, ",")
}

func genesisSummaryByImage(report *scan.Report) map[string]string {
	out := make(map[string]string)
	for _, result := range report.Results {
		counts := map[string]int{}
		for _, img := range result.Images {
			for _, v := range img.Vulnerabilities {
				counts[v.SeverityString]++
			}
		}
		var parts []string
		for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"} {
			if n := counts[sev]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", sev, n))
			}
		}
		if len(parts) > 0 {
			out[result.Reference] = strings.Join(parts, ",")
		}
	}
	return out
}

func genesisScanTotalCounts(report *scan.Report) map[string]int {
	totals := map[string]int{}
	for _, result := range report.Results {
		for _, img := range result.Images {
			for _, v := range img.Vulnerabilities {
				totals[v.SeverityString]++
			}
		}
	}
	return totals
}

func (cc *genesisCmd) saveSlice(ctx context.Context, name string, data []string) error {
	if err := utils.CheckFileExistsPrompt(ctx, name, cc.autoYes); err != nil {
		return err
	}

	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(data, "\n"))
	if err != nil {
		return err
	}
	return nil
}

// RunScanOptions holds options for RunScanWithOptions (Genesis serve API).
type RunScanOptions struct {
	InsecureSkipTLS bool
	Jobs            int
	Timeout         time.Duration
	OnProgress      func(phase string, done, total int)
}

// RunScanWithOptions runs Trivy vulnerability scan on the given image list using
// insecure policy and optional TLS skip. Used by the Genesis serve API when no
// genesisCmd is available (e.g. POST /api/scan).
func RunScanWithOptions(ctx context.Context, images []string, opts RunScanOptions) (*scan.Report, error) {
	if opts.Jobs < 1 || opts.Jobs > utils.MaxWorkerNum {
		opts.Jobs = 1
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	// debug=true so "Start to scan image" etc. appear in server logs for the UI
	scan.InitTrivyLogOutput(true, false)
	if opts.OnProgress != nil {
		opts.OnProgress("Initializing vulnerability scan…", 0, len(images))
	}
	if err := scan.InitTrivyDatabase(ctx, scan.DBOptions{
		CacheDirectory:        utils.TrivyCacheDir(),
		InsecureSkipTLSVerify: opts.InsecureSkipTLS,
	}); err != nil {
		return nil, fmt.Errorf("init trivy database: %w", err)
	}
	if opts.OnProgress != nil {
		opts.OnProgress("Preparing Trivy database…", 0, len(images))
	}
	if err := scan.InitScanner(ctx, scan.ScannerOption{
		Format:                "csv",
		Scanners:              []string{"vuln"},
		CacheDirectory:        utils.TrivyCacheDir(),
		InsecureSkipTLSVerify: opts.InsecureSkipTLS,
	}); err != nil {
		return nil, fmt.Errorf("init scanner: %w", err)
	}
	sysCtx := &types.SystemContext{
		DockerRegistryUserAgent:     utils.DefaultUserAgent(),
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(opts.InsecureSkipTLS),
		OCIInsecureSkipTLSVerify:    opts.InsecureSkipTLS,
	}
	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
		Transports: make(map[string]signature.PolicyTransportScopes),
	}
	report := scan.NewReport()
	if opts.OnProgress != nil {
		opts.OnProgress("Scanning container images…", 0, len(images))
	}
	s, err := hangar.NewScanner(&hangar.ScannerOpts{
		CommonOpts: hangar.CommonOpts{
			Images:              images,
			Arch:                []string{"amd64", "arm64"},
			OS:                  []string{"linux"},
			Timeout:             opts.Timeout,
			Workers:             opts.Jobs,
			FailedImageListName: "",
			SystemContext:       sysCtx,
			Policy:              policy,
		},
		Report:   report,
		Registry: "",
		OnImageComplete: func(done, total int) {
			if opts.OnProgress != nil {
				opts.OnProgress("Scanning container images…", done, total)
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new scanner: %w", err)
	}
	if err := s.Run(ctx); err != nil {
		return report, err
	}
	return report, nil
}

// runScanForGenerateList runs the vulnerability scanner on the given image list
// (when --scan). It inits Trivy DB and scanner, then runs hangar.Scanner.
func (cc *genesisCmd) runScanForGenerateList(ctx context.Context, images []string) (*scan.Report, error) {
	scan.InitTrivyLogOutput(cc.debug, !cc.debug)
	if err := scan.InitTrivyDatabase(ctx, scan.DBOptions{
		CacheDirectory:        utils.TrivyCacheDir(),
		InsecureSkipTLSVerify: !cc.tlsVerify,
	}); err != nil {
		return nil, fmt.Errorf("init trivy database: %w", err)
	}
	if err := scan.InitScanner(ctx, scan.ScannerOption{
		Format:                "csv",
		Scanners:              []string{"vuln"},
		CacheDirectory:        utils.TrivyCacheDir(),
		InsecureSkipTLSVerify: !cc.tlsVerify,
	}); err != nil {
		return nil, fmt.Errorf("init scanner: %w", err)
	}
	sysCtx := cc.baseCmd.newSystemContext()
	sysCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!cc.tlsVerify)
	sysCtx.OCIInsecureSkipTLSVerify = !cc.tlsVerify
	policy, err := cc.baseCmd.getPolicy()
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	jobs := cc.scanJobs
	if jobs < 1 || jobs > utils.MaxWorkerNum {
		jobs = 1
	}
	report := scan.NewReport()
	s, err := hangar.NewScanner(&hangar.ScannerOpts{
		CommonOpts: hangar.CommonOpts{
			Images:              images,
			Arch:                []string{"amd64", "arm64"},
			OS:                  []string{"linux"},
			Timeout:             cc.scanTimeout,
			Workers:             jobs,
			FailedImageListName: "",
			SystemContext:       sysCtx,
			Policy:              policy,
		},
		Report:   report,
		Registry: cc.registry,
	})
	if err != nil {
		return nil, fmt.Errorf("new scanner: %w", err)
	}
	if err := s.Run(ctx); err != nil {
		return report, err // return report so we still have partial results
	}
	return report, nil
}

func (cc *genesisCmd) saveScanReport(ctx context.Context, report *scan.Report, path string) error {
	if err := utils.CheckFileExistsPrompt(ctx, path, cc.autoYes); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return report.WriteCSV(f)
}
