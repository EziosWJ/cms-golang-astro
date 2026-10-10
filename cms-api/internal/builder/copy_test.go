package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeExcludesSymlinkedDependencies(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	dependencies := filepath.Join(root, "dependencies")
	workspace := filepath.Join(root, "workspace")
	for _, path := range []string{site, dependencies} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("template"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dependencies, filepath.Join(site, "node_modules")); err != nil {
		t.Fatal(err)
	}

	if err := copyTree(context.Background(), site, workspace, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(workspace, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("copied node_modules symlink into workspace: lstat error = %v", err)
	}
	if err := os.Symlink(dependencies, filepath.Join(workspace, "node_modules")); err != nil {
		t.Fatalf("workspace dependencies symlink: %v", err)
	}
}

func TestPrepareWorkspaceReplacesStaleWorkspace(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "site")
	dependencies := filepath.Join(root, "dependencies")
	staleDependencies := filepath.Join(root, "stale-dependencies")
	workspace := filepath.Join(root, "generated", "attempt", "site")
	for _, path := range []string{site, dependencies, staleDependencies, workspace} {
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("template"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "stale.txt"), []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dependencies, filepath.Join(site, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(staleDependencies, filepath.Join(workspace, "node_modules")); err != nil {
		t.Fatal(err)
	}

	if err := prepareWorkspace(context.Background(), site, workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(workspace, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale workspace file remains: lstat error = %v", err)
	}
	link, err := os.Readlink(filepath.Join(workspace, "node_modules"))
	if err != nil {
		t.Fatal(err)
	}
	wantLink := filepath.Join(site, "node_modules")
	if link != wantLink {
		t.Fatalf("node_modules points to %q, want %q", link, wantLink)
	}
}
