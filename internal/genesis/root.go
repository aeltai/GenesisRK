package genesis

import (
	"fmt"
	"os"

	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// NewRootCommand returns the root cobra command for the genesisrk binary.
func NewRootCommand() *cobra.Command {
	cc := newGenesisCmd()
	root := cc.baseCmd.cmd
	root.Use = "genesisrk"
	root.Short = "GenesisRK — generate Rancher image lists for air-gapped deployments"
	root.Long = `GenesisRK generates image lists and Kubernetes version lists
for Rancher air-gapped deployments from KDM data and chart repositories.

Built on top of Hangar (https://github.com/cnrancher/hangar).

Subcommands:
  generate  Config-driven, non-interactive (CI-friendly)
  tui       Interactive terminal UI
  serve     REST API and web UI
  version   Print version information`
	root.RunE = nil
	root.PreRun = nil

	addSharedPersistentFlags(root, cc)

	generate := &cobra.Command{
		Use:   "generate",
		Short: "Generate image lists from a YAML config file",
		Long:  "Non-interactive mode ideal for CI and scripts. Requires --rancher and --config.",
		PreRun: func(cmd *cobra.Command, args []string) {
			preRunHero(cc)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cc.configFile == "" {
				return fmt.Errorf("--config is required for generate mode\nExample: genesisrk generate --rancher=v2.13.1 --config=config.yaml")
			}
			cc.interactive = false
			cc.tui = false
			return cc.runGenesisPipeline()
		},
	}
	addGenerateFlags(generate, cc)

	tui := &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI for selecting distros, charts, and versions",
		PreRun: func(cmd *cobra.Command, args []string) {
			preRunHero(cc)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cc.interactive = true
			cc.tui = true
			return cc.runGenesisPipeline()
		},
	}
	addGenerateFlags(tui, cc)

	version := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("genesisrk %s (hangar %s)\n", Version, HangarVersion)
		},
	}

	root.AddCommand(generate, tui, version)
	return root
}

func preRunHero(cc *genesisCmd) {
	utils.SetupLogrus(cc.hideLogTime)
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Print("\033[2J\033[H")
	}
	fmt.Println()
	drawHero()
	fmt.Print("\033[33m")
	fmt.Println("GenesisRK — Generate image lists for Rancher air-gapped deployments.")
	fmt.Println("  • Charts & KDM: Community or Rancher Prime")
	fmt.Println("  • Distros: K3s, RKE2, RKE1 — select versions, CNI, load balancer")
	fmt.Println("  • Output: images.txt, versions.txt; optional scan and Hauler manifest")
	fmt.Print("\033[0m")
	fmt.Println()
	if cc.debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func (cc *genesisCmd) runGenesisPipeline() error {
	if cc.tui {
		cc.interactive = true
	}
	if !cc.interactive && cc.configFile == "" {
		return fmt.Errorf("genesisrk requires either tui mode or --config file")
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
	if cc.interactive && cc.configFile == "" {
		if err := cc.interactivePostRunPrompt(); err != nil {
			return err
		}
	} else if cc.configFile != "" {
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
}

func addSharedPersistentFlags(cmd *cobra.Command, cc *genesisCmd) {
	flags := cmd.PersistentFlags()
	flags.BoolVarP(&cc.debug, "debug", "d", false, "enable debug output")
	flags.StringVarP(&cc.policyPath, "policy", "", "", "path to signature verification policy file")
	flags.BoolVarP(&cc.insecurePolicy, "insecure-policy", "", false, "use allow-everything signature policy")
	flags.BoolVarP(&cc.hideLogTime, "hide-log-time", "", false, "hide timestamp in log output")
}

func addGenerateFlags(cmd *cobra.Command, cc *genesisCmd) {
	flags := cmd.Flags()
	flags.StringVarP(&cc.registry, "registry", "", "", "customize registry URL in generated image list")
	flags.StringVarP(&cc.output, "output", "o", "", "output linux image list file")
	flags.StringVarP(&cc.outputWindows, "output-windows", "", "", "output windows image list")
	flags.StringVarP(&cc.outputSource, "output-source", "", "", "output image list with sources")
	flags.StringVarP(&cc.outputVersions, "output-versions", "", "", "output k8s versions file")
	flags.StringVarP(&cc.rancherVersion, "rancher", "", "", "rancher version (required)")
	flags.StringVarP(&cc.minKubeVersion, "min-kube-version", "", "", "min RKE1 kube version")
	flags.BoolVarP(&cc.dev, "dev", "", false, "use dev branch/URL for charts and KDM")
	flags.StringVarP(&cc.kdm, "kdm", "", "", "KDM file path or URL")
	flags.StringSliceVarP(&cc.charts, "chart", "", nil, "cloned chart repo path")
	flags.StringSliceVarP(&cc.systemCharts, "system-chart", "", nil, "cloned system chart repo path")
	flags.BoolVarP(&cc.kdmRemoveDeprecated, "kdm-remove-deprecated", "", true, "remove deprecated k3s/rke2 versions")
	flags.StringVarP(&cc.rke1Images, "rke-images", "", "", "output RKE1 image list")
	flags.StringVarP(&cc.rke2Images, "rke2-images", "", "", "output RKE2 image list")
	flags.StringVarP(&cc.rke2WindowsImages, "rke2-windows-images", "", "", "output RKE2 Windows image list")
	flags.StringVarP(&cc.k3sImages, "k3s-images", "", "", "output K3s image list")
	flags.BoolVarP(&cc.tlsVerify, "tls-verify", "", true, "require HTTPS and verify certificates")
	flags.BoolVarP(&cc.autoYes, "auto-yes", "y", false, "answer yes automatically")
	flags.BoolVarP(&cc.interactive, "interactive", "i", false, "interactive text mode")
	flags.BoolVarP(&cc.tui, "tui", "", false, "terminal UI mode")
	flags.BoolVarP(&cc.scan, "scan", "", false, "run vulnerability scan on images")
	flags.IntVarP(&cc.scanJobs, "scan-jobs", "", 1, "scan worker count (1-20)")
	flags.DurationVarP(&cc.scanTimeout, "scan-timeout", "", 0, "timeout per image when scanning")
	flags.StringVarP(&cc.scanReport, "scan-report", "", "", "scan report output path")
	flags.StringVarP(&cc.configFile, "config", "c", "", "YAML config file")
	flags.StringVarP(&cc.saveConfigFile, "save-config", "", "", "save TUI selections to YAML config")
	flags.BoolVarP(&cc.exportHauler, "hauler", "", false, "also write a Hauler Images manifest YAML")
	flags.StringVarP(&cc.haulerOutput, "hauler-output", "", "", "Hauler manifest output path (default: [version]-hauler.yaml)")
}
