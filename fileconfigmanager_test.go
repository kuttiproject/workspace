package workspace_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuttiproject/workspace"
)

const cfmtestfile = "cfmtest.json"

var corruptname = regexp.MustCompile(`^cfmtest\.json\.\d{8}T\d{6}\.\d{3}Z\.corrupt$`)

func cfmsetup(t *testing.T) (confdir string, config *sampledata) {
	t.Helper()

	if err := workspace.Set(t.TempDir()); err != nil {
		t.Fatalf("setting workspace failed with: %v", err)
	}
	t.Cleanup(workspace.Reset)

	confdir, err := workspace.ConfigDir()
	if err != nil {
		t.Fatalf("getting config dir failed with: %v", err)
	}

	return confdir, &sampledata{}
}

func corruptfiles(t *testing.T, confdir string) []string {
	t.Helper()

	entries, err := os.ReadDir(confdir)
	if err != nil {
		t.Fatal(err)
	}

	var result []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".corrupt") {
			result = append(result, entry.Name())
		}
	}
	return result
}

// Readers must see either the old or the new content, never a partial
// file, and no temporary files may remain.
func TestConfigSaveIsAtomic(t *testing.T) {
	confdir, config := cfmsetup(t)

	fcm, err := workspace.NewFileConfigManager(cfmtestfile, config)
	if err != nil {
		t.Fatalf("creating config manager failed with: %v", err)
	}

	path := filepath.Join(confdir, cfmtestfile)
	padding := strings.Repeat("x", 1<<20) // Large, to make partial writes likely.

	stop := make(chan struct{})
	var readers sync.WaitGroup
	var readerr error
	var reads int
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}

			data, err := readfileretrying(path)
			if err != nil {
				readerr = err
				return
			}
			reads++

			var parsed sampledata
			if err := json.Unmarshal(data, &parsed); err != nil {
				readerr = err
				return
			}
		}
	}()

	for i := 0; i < 100; i++ {
		config.Name = padding
		config.Age = i
		if err := fcm.Save(); err != nil {
			t.Fatalf("save %d failed with: %v", i, err)
		}
	}

	close(stop)
	readers.Wait()

	if readerr != nil {
		t.Errorf("a reader saw a missing, partial or invalid file after %d reads: %v", reads, readerr)
	}
	t.Logf("file was read %d times while it was being saved", reads)

	entries, _ := os.ReadDir(confdir)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestLoadSetsAsideCorruptFile(t *testing.T) {
	confdir, config := cfmsetup(t)
	path := filepath.Join(confdir, cfmtestfile)

	corrupt := []byte(`{"Name": "precious", "Ag`)
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	fcm, err := workspace.NewFileConfigManager(cfmtestfile, config)
	if err != nil {
		t.Fatalf("a corrupt file should be recovered from, not returned as an error: %v", err)
	}
	if fcm == nil {
		t.Fatal("config manager is nil")
	}

	if config.Name != "Test" || config.Age != 42 {
		t.Errorf("default values should be in use. Got: %#v", config)
	}

	// The file now holds the defaults.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onfile sampledata
	if err := json.Unmarshal(data, &onfile); err != nil || onfile.Name != "Test" {
		t.Errorf("config file should hold valid defaults. Got %q (%v)", data, err)
	}

	// The corrupt original is kept, byte for byte, under the right name.
	backups := corruptfiles(t, confdir)
	if len(backups) != 1 {
		t.Fatalf("expected 1 set-aside file, found %d: %v", len(backups), backups)
	}
	if !corruptname.MatchString(backups[0]) {
		t.Errorf("set-aside file name %q does not match <name>.<timestamp>.corrupt", backups[0])
	}
	kept, err := os.ReadFile(filepath.Join(confdir, backups[0]))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != string(corrupt) {
		t.Errorf("set-aside file content changed. Got: %q", kept)
	}
}

func TestLoadKeepsValidFile(t *testing.T) {
	confdir, config := cfmsetup(t)

	fcm, err := workspace.NewFileConfigManager(cfmtestfile, config)
	if err != nil {
		t.Fatal(err)
	}

	config.Name, config.Age = "Saved", 7
	if err := fcm.Save(); err != nil {
		t.Fatal(err)
	}

	fcm.Reset()
	if err := fcm.Load(); err != nil {
		t.Fatalf("loading a valid file failed with: %v", err)
	}

	if config.Name != "Saved" || config.Age != 7 {
		t.Errorf("saved values were not loaded. Got: %#v", config)
	}
	if backups := corruptfiles(t, confdir); len(backups) != 0 {
		t.Errorf("a valid file should not be set aside. Found: %v", backups)
	}
}

// A second corruption must not overwrite the first set-aside file.
func TestRepeatedCorruptionKeepsEveryBackup(t *testing.T) {
	confdir, config := cfmsetup(t)
	path := filepath.Join(confdir, cfmtestfile)

	fcm, err := workspace.NewFileConfigManager(cfmtestfile, config)
	if err != nil {
		t.Fatal(err)
	}

	for i, content := range []string{`{"first`, `{"second`} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond) // Timestamps have millisecond resolution.

		if err := fcm.Load(); err != nil {
			t.Fatalf("load %d failed with: %v", i, err)
		}
	}

	backups := corruptfiles(t, confdir)
	if len(backups) != 2 {
		t.Fatalf("expected 2 set-aside files, found %d: %v", len(backups), backups)
	}

	var contents []string
	for _, name := range backups {
		data, _ := os.ReadFile(filepath.Join(confdir, name))
		contents = append(contents, string(data))
	}
	if !(contains(contents, `{"first`) && contains(contents, `{"second`)) {
		t.Errorf("an earlier backup was lost. Contents: %q", contents)
	}
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

// readfileretrying reads a file, retrying any error except the file not
// existing. On Windows, opening a file while it is being replaced can
// fail briefly with a sharing violation. That is not a missing or a
// partial file, which are what these tests look for.
func readfileretrying(path string) ([]byte, error) {
	var err error
	for attempt := 0; attempt < 200; attempt++ {
		var data []byte
		data, err = os.ReadFile(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return data, err
		}
		time.Sleep(5 * time.Millisecond)
	}

	return nil, err
}
