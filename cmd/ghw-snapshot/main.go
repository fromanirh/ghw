// +build linux
//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jaypipes/ghw/pkg/snapshot"
)

var (
	// version of application at compile time (-X 'main.version=$(VERSION)').
	version = "(Unknown Version)"
	// buildHash GIT hash of application at compile time (-X 'main.buildHash=$(GITCOMMIT)').
	buildHash = "No Git-hash Provided."
	// buildDate of application at compile time (-X 'main.buildDate=$(BUILDDATE)').
	buildDate = "No Build Date Provided."
	// show debug output
	debug = false
	// output filepath to save snapshot to
	outPath string
)

var rootCmd = &cobra.Command{
	Use:   "ghw-snapshot",
	Short: "ghw-snapshot - Snapshot filesystem containing system information.",
	RunE:  execute,
}

func trace(msg string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(msg, args...)
}

func systemFingerprint() string {
	hn, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	m := md5.New()
	io.WriteString(m, hn)
	return fmt.Sprintf("%x", m.Sum(nil))
}

func defaultOutPath() string {
	fp := systemFingerprint()
	return fmt.Sprintf("%s-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH, fp)
}

func execute(cmd *cobra.Command, args []string) error {
	scratchDir, err := ioutil.TempDir("", "ghw-snapshot")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratchDir)

	snapshot.SetTraceFunction(trace)
	if err = snapshot.CloneTreeInto(scratchDir); err != nil {
		return err
	}
	return doSnapshot(scratchDir)
}

func doSnapshot(buildDir string) error {
	if outPath == "" {
		outPath = defaultOutPath()
		trace("using default output filepath %s\n", outPath)
	}

	var f *os.File
	var err error

	if _, err = os.Stat(outPath); os.IsNotExist(err) {
		if f, err = os.Create(outPath); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		f, err := os.OpenFile(outPath, os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		fs, err := f.Stat()
		if err != nil {
			return err
		}
		if fs.Size() > 0 {
			return fmt.Errorf("File %s already exists and is of size >0", outPath)
		}
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return createSnapshot(tw, buildDir)
}

func main() {
	rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&outPath,
		"out", "o",
		outPath,
		"Path to place snapshot. Defaults to file in current directory with name $OS-$ARCH-$HASHSYSTEMNAME.tar.gz",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false, "Enable or disable debug mode",
	)
}

func createSnapshot(tw *tar.Writer, buildDir string) error {
	return filepath.Walk(buildDir, func(path string, fi os.FileInfo, err error) error {
		if path == buildDir {
			return nil
		}
		var link string

		if fi.Mode()&os.ModeSymlink != 0 {
			trace("processing symlink %s\n", path)
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = strings.TrimPrefix(strings.TrimPrefix(path, buildDir), string(os.PathSeparator))

		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err = io.Copy(tw, f); err != nil {
				return err
			}
			f.Close()
		}
		return nil
	})
}
