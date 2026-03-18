package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSymlinkConfigDefaults(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".bare"), 0o755); err != nil {
		t.Fatal(err)
	}

	paths, err := parseSymlinkConfig(repoDir)
	if err != nil {
		t.Fatalf("parseSymlinkConfig() error = %v", err)
	}

	if len(paths) != len(defaultSymlinkPaths) {
		t.Fatalf("expected %d paths, got %d", len(defaultSymlinkPaths), len(paths))
	}
	for i, p := range paths {
		if p != defaultSymlinkPaths[i] {
			t.Errorf("path[%d] = %q, want %q", i, p, defaultSymlinkPaths[i])
		}
	}
}

func TestParseSymlinkConfigCustom(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".bare"), 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `# This is a comment
.idea/runConfigurations/

.idea/httpRequests/
.run/
# Another comment
.idea/sqldialects.xml
`
	if err := os.WriteFile(filepath.Join(repoDir, ".worktree-symlinks"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := parseSymlinkConfig(repoDir)
	if err != nil {
		t.Fatalf("parseSymlinkConfig() error = %v", err)
	}

	expected := []string{
		".idea/runConfigurations/",
		".idea/httpRequests/",
		".run/",
		".idea/sqldialects.xml",
	}

	if len(paths) != len(expected) {
		t.Fatalf("expected %d paths, got %d: %v", len(expected), len(paths), paths)
	}
	for i, p := range paths {
		if p != expected[i] {
			t.Errorf("path[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestEnsureSharedDirs(t *testing.T) {
	repoDir := t.TempDir()

	paths := []string{".idea/runConfigurations/", ".idea/dataSources.xml", ".run/"}
	if err := ensureSharedDirs(repoDir, paths); err != nil {
		t.Fatalf("ensureSharedDirs() error = %v", err)
	}

	// .idea/runConfigurations/ → .idea-shared/runConfigurations/ (directory)
	runConfigDir := filepath.Join(repoDir, ".idea-shared", "runConfigurations")
	fi, err := os.Stat(runConfigDir)
	if err != nil {
		t.Fatalf(".idea-shared/runConfigurations/ does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Error(".idea-shared/runConfigurations/ is not a directory")
	}

	// .idea/dataSources.xml → .idea-shared/ parent exists, but file does not
	ideaSharedDir := filepath.Join(repoDir, ".idea-shared")
	fi, err = os.Stat(ideaSharedDir)
	if err != nil {
		t.Fatalf(".idea-shared/ does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Error(".idea-shared/ is not a directory")
	}
	dataSourcesFile := filepath.Join(repoDir, ".idea-shared", "dataSources.xml")
	if _, err := os.Stat(dataSourcesFile); err == nil {
		t.Error("dataSources.xml should NOT exist as a file (only parent dir should be created)")
	}

	// .run/ → .run/ (directory, used as-is)
	runDir := filepath.Join(repoDir, ".run")
	fi, err = os.Stat(runDir)
	if err != nil {
		t.Fatalf(".run/ does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Error(".run/ is not a directory")
	}
}

func TestCreateSymlinks(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	// Create structure
	for _, dir := range []string{
		".bare",
		".idea-shared/runConfigurations",
		".run",
		"wt1",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	paths := []string{".idea/runConfigurations/", ".run/"}
	if err := createSymlinks(repoDir, worktreePath, paths); err != nil {
		t.Fatalf("createSymlinks() error = %v", err)
	}

	// Verify .idea/runConfigurations is a symlink
	runConfigLink := filepath.Join(worktreePath, ".idea", "runConfigurations")
	fi, err := os.Lstat(runConfigLink)
	if err != nil {
		t.Fatalf(".idea/runConfigurations does not exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error(".idea/runConfigurations is not a symlink")
	}

	// Verify .run is a symlink
	runLink := filepath.Join(worktreePath, ".run")
	fi, err = os.Lstat(runLink)
	if err != nil {
		t.Fatalf(".run does not exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error(".run is not a symlink")
	}

	// Verify relative symlink targets
	target, err := os.Readlink(runConfigLink)
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("..", "..", ".idea-shared", "runConfigurations") {
		t.Errorf(".idea/runConfigurations target = %q, want %q", target, filepath.Join("..", "..", ".idea-shared", "runConfigurations"))
	}

	target, err = os.Readlink(runLink)
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("..", ".run") {
		t.Errorf(".run target = %q, want %q", target, filepath.Join("..", ".run"))
	}

	// Verify reading through symlinks works (target dirs exist)
	entries, err := os.ReadDir(runConfigLink)
	if err != nil {
		t.Errorf("failed to read through .idea/runConfigurations symlink: %v", err)
	}
	_ = entries // empty but accessible

	entries, err = os.ReadDir(runLink)
	if err != nil {
		t.Errorf("failed to read through .run symlink: %v", err)
	}
	_ = entries
}

func TestCreateSymlinksSkipsExistingSymlink(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	for _, dir := range []string{
		".bare",
		".idea-shared/runConfigurations",
		"wt1/.idea",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Manually create a symlink pointing somewhere else
	originalTarget := "/some/nonexistent/path"
	symlinkPath := filepath.Join(worktreePath, ".idea", "runConfigurations")
	if err := os.Symlink(originalTarget, symlinkPath); err != nil {
		t.Fatal(err)
	}

	// Call createSymlinks
	if err := createSymlinks(repoDir, worktreePath, []string{".idea/runConfigurations/"}); err != nil {
		t.Fatalf("createSymlinks() error = %v", err)
	}

	// Verify symlink was NOT modified
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != originalTarget {
		t.Errorf("symlink was modified: target = %q, want %q", target, originalTarget)
	}
}

func TestCreateSymlinksSkipsExistingDir(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	for _, dir := range []string{
		".bare",
		".idea-shared/runConfigurations",
		"wt1/.idea/runConfigurations",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Call createSymlinks — should skip because real directory exists
	if err := createSymlinks(repoDir, worktreePath, []string{".idea/runConfigurations/"}); err != nil {
		t.Fatalf("createSymlinks() error = %v", err)
	}

	// Verify it's still a real directory, NOT a symlink
	fi, err := os.Lstat(filepath.Join(worktreePath, ".idea", "runConfigurations"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("expected real directory, got symlink")
	}
	if !fi.IsDir() {
		t.Error("expected a directory")
	}
}

func TestCreateSymlinksSkipsExistingFile(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	for _, dir := range []string{
		".bare",
		".idea-shared",
		"wt1/.idea",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a real file at the destination
	filePath := filepath.Join(worktreePath, ".idea", "dataSources.xml")
	if err := os.WriteFile(filePath, []byte("<xml>original</xml>"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Call createSymlinks — should skip because real file exists
	if err := createSymlinks(repoDir, worktreePath, []string{".idea/dataSources.xml"}); err != nil {
		t.Fatalf("createSymlinks() error = %v", err)
	}

	// Verify it's still a real file, NOT a symlink
	fi, err := os.Lstat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Error("expected real file, got symlink")
	}
	if fi.IsDir() {
		t.Error("expected a file, got directory")
	}

	// Verify content unchanged
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "<xml>original</xml>" {
		t.Errorf("file content modified: %q", content)
	}
}

func TestSetupSymlinksNoBareDir(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatal(err)
	}

	// No .bare/ directory — should return nil silently
	err := SetupSymlinks(repoDir, worktreePath)
	if err != nil {
		t.Fatalf("SetupSymlinks() error = %v, want nil", err)
	}

	// Verify no .idea-shared/ was created
	if _, err := os.Stat(filepath.Join(repoDir, ".idea-shared")); !os.IsNotExist(err) {
		t.Error(".idea-shared/ should not have been created")
	}
}

func TestSetupSymlinksEndToEnd(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	for _, dir := range []string{".bare", "wt1"} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// First call
	if err := SetupSymlinks(repoDir, worktreePath); err != nil {
		t.Fatalf("SetupSymlinks() error = %v", err)
	}

	// Verify .idea-shared/ directories were created
	for _, subdir := range []string{
		".idea-shared/runConfigurations",
		".idea-shared/codeStyles",
		".idea-shared/inspectionProfiles",
		".idea-shared/scopes",
	} {
		fi, err := os.Stat(filepath.Join(repoDir, subdir))
		if err != nil {
			t.Errorf("%s does not exist: %v", subdir, err)
			continue
		}
		if !fi.IsDir() {
			t.Errorf("%s is not a directory", subdir)
		}
	}

	// Verify .idea-shared/ parent exists for file entries
	fi, err := os.Stat(filepath.Join(repoDir, ".idea-shared"))
	if err != nil {
		t.Fatalf(".idea-shared/ does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Error(".idea-shared/ is not a directory")
	}

	// Verify .run/ was created in repoDir
	fi, err = os.Stat(filepath.Join(repoDir, ".run"))
	if err != nil {
		t.Fatalf(".run/ does not exist: %v", err)
	}
	if !fi.IsDir() {
		t.Error(".run/ is not a directory")
	}

	// Verify all default symlinks exist in worktree
	expectedSymlinks := map[string]string{
		".idea/runConfigurations":     filepath.Join("..", "..", ".idea-shared", "runConfigurations"),
		".idea/dataSources.xml":       filepath.Join("..", "..", ".idea-shared", "dataSources.xml"),
		".idea/dataSources.local.xml": filepath.Join("..", "..", ".idea-shared", "dataSources.local.xml"),
		".idea/codeStyles":            filepath.Join("..", "..", ".idea-shared", "codeStyles"),
		".idea/inspectionProfiles":    filepath.Join("..", "..", ".idea-shared", "inspectionProfiles"),
		".idea/scopes":                filepath.Join("..", "..", ".idea-shared", "scopes"),
		".run":                        filepath.Join("..", ".run"),
	}

	for dest, wantTarget := range expectedSymlinks {
		fullPath := filepath.Join(worktreePath, dest)
		fi, err := os.Lstat(fullPath)
		if err != nil {
			t.Errorf("%s does not exist: %v", dest, err)
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", dest)
			continue
		}
		target, err := os.Readlink(fullPath)
		if err != nil {
			t.Errorf("Readlink(%s) error = %v", dest, err)
			continue
		}
		if target != wantTarget {
			t.Errorf("%s target = %q, want %q", dest, target, wantTarget)
		}
	}

	// Idempotency: calling again should not fail
	if err := SetupSymlinks(repoDir, worktreePath); err != nil {
		t.Fatalf("SetupSymlinks() second call error = %v", err)
	}
}

func TestCreateSymlinksFileEntry(t *testing.T) {
	repoDir := t.TempDir()
	worktreePath := filepath.Join(repoDir, "wt1")

	for _, dir := range []string{
		".bare",
		".idea-shared",
		"wt1",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// dataSources.xml file does not exist yet — it will be created by IntelliJ later
	// The symlink should still be created pointing to the (not-yet-existing) shared location
	if err := createSymlinks(repoDir, worktreePath, []string{".idea/dataSources.xml"}); err != nil {
		t.Fatalf("createSymlinks() error = %v", err)
	}

	symlinkPath := filepath.Join(worktreePath, ".idea", "dataSources.xml")
	fi, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf(".idea/dataSources.xml does not exist: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error(".idea/dataSources.xml is not a symlink")
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatal(err)
	}
	wantTarget := filepath.Join("..", "..", ".idea-shared", "dataSources.xml")
	if target != wantTarget {
		t.Errorf("target = %q, want %q", target, wantTarget)
	}
}
