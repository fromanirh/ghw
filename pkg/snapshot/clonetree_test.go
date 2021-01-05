//
// Use and distribution licensed under the Apache license version 2.
//
// See the COPYING file in the root project directory for full text.
//

package snapshot_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jaypipes/ghw/pkg/snapshot"
)

// nolint: gocyclo
func TestCloneTree(t *testing.T) {
	root, err := snapshot.Unpack(testDataSnapshot)
	if err != nil {
		t.Fatalf("Expected nil err, but got %v", err)
	}

	cloneRoot, err := ioutil.TempDir("", "ghw-test-clonetree-*")
	if err != nil {
		t.Fatalf("Expected nil err, but got %v", err)
	}
	defer os.RemoveAll(cloneRoot)

	fileSpecs := []string{
		filepath.Join(root, "ghw-test-*"),
		filepath.Join(root, "different/subtree/ghw*"),
		filepath.Join(root, "nested/ghw-test*"),
		filepath.Join(root, "nested/tree/of/subdirectories/forming/deep/unbalanced/tree/ghw-test-3"),
	}
	err = snapshot.CopyFilesInto(fileSpecs, cloneRoot, nil)
	if err != nil {
		t.Fatalf("Expected nil err, but got %v", err)
	}

	origContent, err := scanTree(root, "", []string{""})
	if err != nil {
		t.Fatalf("Expected nil err, but got %v", err)
	}
	sort.Strings(origContent)

	cloneContent, err := scanTree(cloneRoot, cloneRoot, []string{"", "/tmp"})
	if err != nil {
		t.Fatalf("Expected nil err, but got %v", err)
	}
	sort.Strings(cloneContent)

	if len(origContent) != len(cloneContent) {
		t.Fatalf("Expected tree size %d got %d", len(origContent), len(cloneContent))
	}
	if !reflect.DeepEqual(origContent, cloneContent) {
		t.Fatalf("subtree content different expected %#v got %#v", origContent, cloneContent)
	}
}

func scanTree(root, prefix string, excludeList []string) ([]string, error) {
	var contents []string
	return contents, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fp := strings.TrimPrefix(path, prefix); !includedInto(fp, excludeList) {
			contents = append(contents, fp)
		}
		return nil
	})
}

func includedInto(s string, items []string) bool {
	if items == nil {
		return false
	}
	for _, item := range items {
		if s == item {
			return true
		}
	}
	return false
}
