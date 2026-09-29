package listgenerator

import (
	"fmt"

	"github.com/cnrancher/hangar/pkg/utils"
)

// DefaultCertManagerVersion is the cert-manager release used when generating
// the prerequisite image list for Rancher Helm installs. Rancher does not
// ship cert-manager in rancher/charts; it must be installed from jetstack
// before `helm install rancher`.
//
// Pin intentionally: bump when Rancher install docs recommend a newer line.
const DefaultCertManagerVersion = "v1.16.3"

// CertManagerSource formats the synthetic chart source tag so images show up
// under the cert-manager chart in the Genesis tree.
func CertManagerSource(version string) string {
	if version == "" {
		version = DefaultCertManagerVersion
	}
	return fmt.Sprintf("[cert-manager;cert-manager:%s]", version)
}

// CertManagerImages returns the quay.io/jetstack images for the given
// cert-manager release (controller, webhook, cainjector, acmesolver,
// startupapicheck).
func CertManagerImages(version string) []string {
	if version == "" {
		version = DefaultCertManagerVersion
	}
	comps := []string{
		"cert-manager-controller",
		"cert-manager-webhook",
		"cert-manager-cainjector",
		"cert-manager-acmesolver",
		"cert-manager-startupapicheck",
	}
	out := make([]string, 0, len(comps))
	for _, c := range comps {
		out = append(out, fmt.Sprintf("quay.io/jetstack/%s:%s", c, version))
	}
	return out
}

// MergeCertManagerImages adds jetstack cert-manager images into linuxImages
// with a synthetic chart source so they appear as the cert-manager chart in
// selection UIs. No-op when linuxImages is nil.
func MergeCertManagerImages(linuxImages map[string]map[string]bool, version string) {
	if linuxImages == nil {
		return
	}
	source := CertManagerSource(version)
	for _, img := range CertManagerImages(version) {
		utils.AddSourceToImage(linuxImages, img, source)
	}
}
