//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package snapshot

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	pciaddr "github.com/jaypipes/ghw/pkg/pci/address"
)

// PCIDevicesCloneContent return a slice of glob patterns which represent the pseudofiles
// ghw cares about, pertaining to PCI devices only.
// Beware: the content is host-specific, because the PCI topology is host-dependent and unpredictable.
func PCIDevicesCloneContent() []string {
	var fileSpecs []string
	pciRoots := []string{
		"/sys/bus/pci/devices",
	}
	for {
		if len(pciRoots) == 0 {
			break
		}
		pciRoot := pciRoots[0]
		pciRoots = pciRoots[1:]
		specs, roots := scanPCIDeviceRoot(pciRoot)
		pciRoots = append(pciRoots, roots...)
		fileSpecs = append(fileSpecs, specs...)
	}
	return fileSpecs
}

// scanPCIDeviceRoot reports a slice of glob patterns which represent the pseudofiles
// ghw cares about pertaining to all the PCI devices connected to the bus connected from the
// given root; usually (but not always) a CPU packages has 1+ PCI(e) roots, forming the first
// level; more PCI bridges are (usually) attached to this level, creating deep nested trees.
// hence we need to scan all possible roots, to make sure not to miss important devices.
func scanPCIDeviceRoot(root string) (fileSpecs []string, pciRoots []string) {
	trace("scanning PCI device root %q\n", root)

	perDevEntries := []string{
		"class",
		"device",
		"irq",
		"local_cpulist",
		"modalias",
		"numa_node",
		"revision",
		"vendor",
	}
	entries, err := ioutil.ReadDir(root)
	if err != nil {
		return []string{}, []string{}
	}
	for _, entry := range entries {
		entryName := entry.Name()
		if addr := pciaddr.FromString(entryName); addr == nil {
			// doesn't look like a entry we care about
			continue
		}

		entryPath := filepath.Join(root, entryName)
		pciEntry := findPCIEntryFromPath(root, entryName)
		trace("PCI entry is %q\n", pciEntry)
		fileSpecs = append(fileSpecs, entryPath)
		for _, perNetEntry := range perDevEntries {
			fileSpecs = append(fileSpecs, filepath.Join(pciEntry, perNetEntry))
		}

		if isPCIBridge(entryPath) {
			trace("adding new PCI root %q\n", entryName)
			pciRoots = append(pciRoots, pciEntry)
		}
	}
	return fileSpecs, pciRoots
}

func findPCIEntryFromPath(root, entryName string) string {
	entryPath := filepath.Join(root, entryName)
	fi, err := os.Lstat(entryPath)
	if err != nil {
		trace("stat(%s) failed: %v\n", entryPath, err)
		return entryPath // what else we can do?
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		// regular file, nothing to resolve
		return entryPath
	}
	// resolve symlink
	target, err := os.Readlink(entryPath)
	trace("entry %q is symlink resolved to %q\n", entryPath, target)
	if err != nil {
		trace("readlink(%s) failed: %v - skipped\n", entryPath, err)
		return entryPath // what else we can do?
	}
	return filepath.Clean(filepath.Join(root, target))
}

const (
	//        subclass ---++
	// class -----------++||
	//                  VVVV
	PCI_BRIDGE_PCI = "0x060400"
)

// this is a hack to avoid this circolar import:
// snapshot -> pci -> context -> snapshot
func isPCIBridge(entryPath string) bool {
	if data, err := ioutil.ReadFile(filepath.Join(entryPath, "class")); err == nil {
		devClass := strings.TrimSpace(string(data))
		switch devClass {
		// add more bridges once we are sure they are relevant to our use case.
		case PCI_BRIDGE_PCI:
			trace("pci device %q is a pci bridge\n", entryPath)
			return true
		}
	}
	return false
}
