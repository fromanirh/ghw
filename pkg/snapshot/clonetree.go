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
)

func CloneTreeInto(scratchDir string) error {
	var err error

	var createPaths = []string{
		"proc",
		"etc",
		"sys/block",
	}

	for _, path := range createPaths {
		if err = os.MkdirAll(filepath.Join(scratchDir, path), os.ModePerm); err != nil {
			return err
		}
	}

	if err = createPseudofiles(scratchDir); err != nil {
		return err
	}
	if err = createBlockDevices(scratchDir); err != nil {
		return err
	}

	fileSpecs := []string{
		"/sys/bus/pci/devices/*",
		"/sys/devices/pci*/*/irq",
		"/sys/devices/pci*/*/local_cpulist",
		"/sys/devices/pci*/*/modalias",
		"/sys/devices/pci*/*/numa_node",
		"/sys/devices/pci*/pci_bus/*/cpulistaffinity",
		"/sys/devices/system/cpu/cpu*/cache/index*/*",
		"/sys/devices/system/cpu/cpu*/topology/*",
		"/sys/devices/system/memory/block_size_bytes",
		"/sys/devices/system/memory/memory*/online",
		"/sys/devices/system/memory/memory*/state",
		"/sys/devices/system/node/has_*",
		"/sys/devices/system/node/online",
		"/sys/devices/system/node/possible",
		"/sys/devices/system/node/node*/cpu*",
		"/sys/devices/system/node/node*/distance",
	}
	return CopyFilesInto(fileSpecs, scratchDir, nil)
}

// Attempting to tar up pseudofiles like /proc/cpuinfo is an exercise in
// futility. Notably, the pseudofiles, when read by syscalls, do not return the
// number of bytes read. This causes the tar writer to write zero-length files.
//
// Instead, it is necessary to build a directory structure in a tmpdir and
// create actual files with copies of the pseudofile contents
func createPseudofiles(buildDir string) error {
	createPseudofilePaths := []string{
		"/proc/cpuinfo",
		"/proc/meminfo",
		"/etc/mtab",
	}

	for _, path := range createPseudofilePaths {
		err := copyPseudoFile(path, filepath.Join(buildDir, path))
		if err != nil {
			return err
		}
	}
	return nil
}

func copyPseudoFile(path, targetPath string) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	trace("creating %s\n", targetPath)
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	if _, err = f.Write(buf); err != nil {
		return err
	}
	f.Close()
	return nil
}

func createBlockDevices(buildDir string) error {
	// Grab all the block device pseudo-directories from /sys/block symlinks
	// (excluding loopback devices) and inject them into our build filesystem
	// with all but the circular symlink'd subsystem directories
	devLinks, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return err
	}
	for _, devLink := range devLinks {
		dname := devLink.Name()
		if strings.HasPrefix(dname, "loop") {
			continue
		}
		devPath := filepath.Join("/sys/block", dname)
		trace("processing block device %q\n", devPath)

		// from the sysfs layout, we know this is always a symlink
		linkContentPath, err := os.Readlink(devPath)
		if err != nil {
			return err
		}
		trace("link target for block device %q is %q\n", devPath, linkContentPath)

		// Create a symlink in our build filesystem that is a directory
		// pointing to the actual device bus path where the block device's
		// information directory resides
		linkPath := filepath.Join(buildDir, "sys/block", dname)
		linkTargetPath := filepath.Join(
			buildDir,
			"sys/block",
			strings.TrimPrefix(linkContentPath, string(os.PathSeparator)),
		)
		trace("creating device directory %s\n", linkTargetPath)
		if err = os.MkdirAll(linkTargetPath, os.ModePerm); err != nil {
			return err
		}

		trace("linking device directory %s to %s\n", linkPath, linkContentPath)
		// Make sure the link target is a relative path!
		// if we use absolute path, the link target will be an absolute path starting
		// with buildDir, hence the snapshot will contain broken link.
		// Otherwise, the unpack directory will never have the same prefix of buildDir!
		if err = os.Symlink(linkContentPath, linkPath); err != nil {
			return err
		}
		// Now read the source block device directory and populate the
		// newly-created target link in the build directory with the
		// appropriate block device pseudofiles
		srcDeviceDir := filepath.Join(
			"/sys/block",
			strings.TrimPrefix(linkContentPath, string(os.PathSeparator)),
		)
		trace("creating device directory %q from %q\n", linkTargetPath, srcDeviceDir)
		if err = createBlockDeviceDir(linkTargetPath, srcDeviceDir); err != nil {
			return err
		}
	}
	return nil
}

func createBlockDeviceDir(buildDeviceDir string, srcDeviceDir string) error {
	// Populate the supplied directory (in our build filesystem) with all the
	// appropriate information pseudofile contents for the block device.
	devName := filepath.Base(srcDeviceDir)
	devFiles, err := ioutil.ReadDir(srcDeviceDir)
	if err != nil {
		return err
	}
	for _, f := range devFiles {
		fname := f.Name()
		fp := filepath.Join(srcDeviceDir, fname)
		fi, err := os.Lstat(fp)
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			// Ignore any symlinks in the deviceDir since they simply point to
			// either self-referential links or information we aren't
			// interested in like "subsystem"
			continue
		} else if fi.IsDir() {
			if strings.HasPrefix(fname, devName) {
				// We're interested in are the directories that begin with the
				// block device name. These are directories with information
				// about the partitions on the device
				buildPartitionDir := filepath.Join(
					buildDeviceDir, fname,
				)
				srcPartitionDir := filepath.Join(
					srcDeviceDir, fname,
				)
				trace("creating partition directory %s\n", buildPartitionDir)
				err = os.MkdirAll(buildPartitionDir, os.ModePerm)
				if err != nil {
					return err
				}
				err = createPartitionDir(buildPartitionDir, srcPartitionDir)
				if err != nil {
					return err
				}
			}
		} else if fi.Mode().IsRegular() {
			// Regular files in the block device directory are both regular and
			// pseudofiles containing information such as the size (in sectors)
			// and whether the device is read-only
			buf, err := ioutil.ReadFile(fp)
			if err != nil {
				return err
			}
			targetPath := filepath.Join(buildDeviceDir, fname)
			trace("creating %s\n", targetPath)
			f, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err = f.Write(buf); err != nil {
				return err
			}
			f.Close()
		}
	}
	// There is a special file $DEVICE_DIR/queue/rotational that, for some hard
	// drives, contains a 1 or 0 indicating whether the device is a spinning
	// disk or not
	srcQueueDir := filepath.Join(
		srcDeviceDir,
		"queue",
	)
	buildQueueDir := filepath.Join(
		buildDeviceDir,
		"queue",
	)
	err = os.MkdirAll(buildQueueDir, os.ModePerm)
	if err != nil {
		return err
	}
	fp := filepath.Join(srcQueueDir, "rotational")
	buf, err := ioutil.ReadFile(fp)
	if err != nil {
		return err
	}
	targetPath := filepath.Join(buildQueueDir, "rotational")
	trace("creating %s\n", targetPath)
	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	if _, err = f.Write(buf); err != nil {
		return err
	}
	f.Close()

	return nil
}

func createPartitionDir(buildPartitionDir string, srcPartitionDir string) error {
	// Populate the supplied directory (in our build filesystem) with all the
	// appropriate information pseudofile contents for the partition.
	partFiles, err := ioutil.ReadDir(srcPartitionDir)
	if err != nil {
		return err
	}
	for _, f := range partFiles {
		fname := f.Name()
		fp := filepath.Join(srcPartitionDir, fname)
		fi, err := os.Lstat(fp)
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			// Ignore any symlinks in the partition directory since they simply
			// point to information we aren't interested in like "subsystem"
			continue
		} else if fi.IsDir() {
			// The subdirectories in the partition directory are not
			// interesting for us. They have information about power events and
			// traces
			continue
		} else if fi.Mode().IsRegular() {
			// Regular files in the block device directory are both regular and
			// pseudofiles containing information such as the size (in sectors)
			// and whether the device is read-only
			buf, err := ioutil.ReadFile(fp)
			if err != nil {
				return err
			}
			targetPath := filepath.Join(buildPartitionDir, fname)
			trace("creating %s\n", targetPath)
			f, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err = f.Write(buf); err != nil {
				return err
			}
			f.Close()
		}
	}
	return nil
}

type CopyFileOptions struct {
	IsSymlink func(path string, info os.FileInfo) bool
}

func CopyFilesInto(fileSpecs []string, destDir string, opts *CopyFileOptions) error {
	if opts == nil {
		opts = &CopyFileOptions{
			IsSymlink: isSymlink,
		}
	}
	for _, fileSpec := range fileSpecs {
		trace("copying spec: %q\n", fileSpec)
		matches, err := filepath.Glob(fileSpec)
		if err != nil {
			return err
		}
		if err := copyFileTreeInto(matches, destDir, opts); err != nil {
			return err
		}
	}
	return nil
}

func copyFileTreeInto(paths []string, destDir string, opts *CopyFileOptions) error {
	for _, path := range paths {
		trace("  copying path: %q\n", path)
		baseDir := filepath.Dir(path)
		if err := os.MkdirAll(filepath.Join(destDir, baseDir), os.ModePerm); err != nil {
			return err
		}

		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if opts.IsSymlink(path, fi) {
			trace("    copying link: %q\n", path)
			if err := copyLink(path, filepath.Join(destDir, path)); err != nil {
				return err
			}
		} else {
			trace("    copying file: %q\n", path)
			if err := copyPseudoFile(path, filepath.Join(destDir, path)); err != nil {
				return err
			}
		}
	}
	return nil
}

func isSymlink(path string, fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}

func copyLink(path, targetPath string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	if err := os.Symlink(target, targetPath); err != nil {
		return err
	}

	return nil
}
