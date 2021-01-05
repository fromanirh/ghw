//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PackOptions struct {
	Compress bool
}

func PackFrom(snapshotName, sourceRoot string, opts PackOptions) error {
	var f *os.File
	var err error

	if _, err = os.Stat(snapshotName); os.IsNotExist(err) {
		if f, err = os.Create(snapshotName); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		f, err := os.OpenFile(snapshotName, os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		fs, err := f.Stat()
		if err != nil {
			return err
		}
		if fs.Size() > 0 {
			return fmt.Errorf("File %s already exists and is of size >0", snapshotName)
		}
	}
	defer f.Close()

	var dst io.Writer
	if opts.Compress {
		trace("using gzip compression")
		gzw := gzip.NewWriter(f)
		defer gzw.Close()
		dst = gzw
	} else {
		dst = f
	}

	tw := tar.NewWriter(dst)
	defer tw.Close()

	return createSnapshot(tw, sourceRoot)
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
