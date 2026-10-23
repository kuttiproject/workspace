package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writesource(t *testing.T, dir string, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

// failingreader returns n bytes, then fails.
type failingreader struct {
	n   int
	err error
}

func (fr *failingreader) Read(p []byte) (int, error) {
	if fr.n <= 0 {
		return 0, fr.err
	}
	if len(p) > fr.n {
		p = p[:fr.n]
	}
	fr.n -= len(p)
	return len(p), nil
}

func TestCopyReplacesFile(t *testing.T) {
	dir := t.TempDir()
	content := payload(9<<20+123, 1) // Not a multiple of the chunk size.
	src := writesource(t, dir, "src.bin", content)
	dst := writesource(t, dir, "dst.bin", []byte("old"))

	if err := copyfile(src, dst, 131072, true, nil); err != nil {
		t.Fatalf("copy failed with: %v", err)
	}

	if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
		t.Errorf("destination holds %d bytes, expected %d", len(got), len(content))
	}
	nodownloadleftovers(t, dir)

	// Same permissions as a file made with os.Create.
	ref, err := os.Create(filepath.Join(dir, "reference"))
	if err != nil {
		t.Fatal(err)
	}
	ref.Close()
	os.Remove(dst)
	if err := copyfile(src, dst, 131072, true, nil); err != nil {
		t.Fatal(err)
	}
	refinfo, _ := os.Stat(filepath.Join(dir, "reference"))
	dstinfo, _ := os.Stat(dst)
	if refinfo.Mode().Perm() != dstinfo.Mode().Perm() {
		t.Errorf("copied file has mode %v, os.Create gives %v", dstinfo.Mode().Perm(), refinfo.Mode().Perm())
	}
}

func TestCopyReportsProgress(t *testing.T) {
	dir := t.TempDir()
	size := 10<<20 + 5
	src := writesource(t, dir, "src.bin", payload(size, 2))

	var last int64
	var calls int
	err := copyfile(src, filepath.Join(dir, "dst.bin"), 0, true, func(current, total int64) {
		calls++
		if current <= last {
			t.Errorf("progress went from %d to %d", last, current)
		}
		if total != int64(size) {
			t.Errorf("progress total is %d, expected %d", total, size)
		}
		last = current
	})
	if err != nil {
		t.Fatal(err)
	}

	if last != int64(size) || calls < 2 {
		t.Errorf("last progress was %d after %d calls, expected %d", last, calls, size)
	}
}

func TestCopyFailureKeepsDestination(t *testing.T) {
	dir := t.TempDir()
	dst := writesource(t, dir, "dst.bin", []byte("old"))

	boom := errors.New("disk on fire")
	_, err := copytotempfile(&failingreader{n: 6 << 20, err: boom}, 20<<20, dst, 0, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("expected the read error, got: %v", err)
	}

	if got, _ := os.ReadFile(dst); string(got) != "old" {
		t.Errorf("destination changed to %d bytes", len(got))
	}
	nodownloadleftovers(t, dir)
}

func TestCopySourceSizeChangeIsAnError(t *testing.T) {
	dir := t.TempDir()
	dst := writesource(t, dir, "dst.bin", []byte("old"))

	for _, tc := range []struct {
		name     string
		actual   int
		expected int64
	}{
		{"shrunk", 1000, 2000},
		{"grew", 2000, 1000},
	} {
		_, err := copytotempfile(io.LimitReader(bytes.NewReader(payload(tc.actual, 3)), int64(tc.actual)), tc.expected, dst, 0, nil)
		if err == nil || !strings.Contains(err.Error(), "changed") {
			t.Errorf("%s: expected a source-changed error, got: %v", tc.name, err)
		}
	}

	if got, _ := os.ReadFile(dst); string(got) != "old" {
		t.Errorf("destination changed to %d bytes", len(got))
	}
	nodownloadleftovers(t, dir)
}

// Copying a file onto itself used to empty it.
func TestCopyOntoItself(t *testing.T) {
	dir := t.TempDir()
	content := payload(5<<20, 4)
	path := writesource(t, dir, "same.bin", content)

	if err := copyfile(path, path, 131072, true, nil); err != nil {
		t.Fatalf("copy onto itself failed with: %v", err)
	}

	if got, _ := os.ReadFile(path); !bytes.Equal(got, content) {
		t.Errorf("file holds %d bytes after copy onto itself, expected %d", len(got), len(content))
	}
	nodownloadleftovers(t, dir)
}

func TestCopyNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := writesource(t, dir, "src.bin", payload(1000, 5))
	dst := writesource(t, dir, "dst.bin", []byte("old"))

	if err := copyfile(src, dst, 0, false, nil); err == nil {
		t.Fatal("copy over an existing file with overwrite off should have failed")
	}
	if got, _ := os.ReadFile(dst); string(got) != "old" {
		t.Errorf("destination changed to %d bytes", len(got))
	}

	fresh := filepath.Join(dir, "fresh.bin")
	if err := copyfile(src, fresh, 0, false, nil); err != nil {
		t.Errorf("copy to a new file with overwrite off failed with: %v", err)
	}
	nodownloadleftovers(t, dir)
}

func TestCopyEdgeCases(t *testing.T) {
	dir := t.TempDir()

	// Empty source.
	empty := writesource(t, dir, "empty.bin", nil)
	emptydst := filepath.Join(dir, "emptydst.bin")
	if err := copyfile(empty, emptydst, 0, true, nil); err != nil {
		t.Errorf("copying an empty file failed with: %v", err)
	}
	if info, err := os.Stat(emptydst); err != nil || info.Size() != 0 {
		t.Errorf("empty copy is wrong: %v, %v", info, err)
	}

	// Missing source, and a directory as source.
	if err := copyfile(filepath.Join(dir, "nothere"), filepath.Join(dir, "x"), 0, true, nil); err == nil {
		t.Error("copying a missing file should have failed")
	}
	if err := copyfile(dir, filepath.Join(dir, "x"), 0, true, nil); err == nil {
		t.Error("copying a directory should have failed")
	}

	// Zero and negative buffer sizes.
	src := writesource(t, dir, "src.bin", payload(100000, 6))
	for _, size := range []int64{0, -5} {
		dst := filepath.Join(dir, fmt.Sprintf("buf%d.bin", size))
		if err := copyfile(src, dst, size, true, nil); err != nil {
			t.Errorf("buffer size %d failed with: %v", size, err)
		}
	}
	nodownloadleftovers(t, dir)
}

func TestCopyConcurrentToSameFile(t *testing.T) {
	dir := t.TempDir()
	content := payload(5<<20, 7)
	src := writesource(t, dir, "src.bin", content)
	dst := filepath.Join(dir, "dst.bin")

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- copyfile(src, dst, 131072, true, nil)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("a concurrent copy failed with: %v", err)
		}
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, content) {
		t.Errorf("destination is not the expected content (%d bytes)", len(got))
	}
	nodownloadleftovers(t, dir)
}

// While a copy replaces a file, readers see the old file or the new one in
// full. They never find it missing or partial.
func TestCopyReplacementIsAtomic(t *testing.T) {
	dir := t.TempDir()
	oldcontent := payload(1<<20, 8)
	newcontent := payload(9<<20, 9)
	dst := writesource(t, dir, "dst.bin", oldcontent)
	src := writesource(t, dir, "src.bin", newcontent)

	stop := make(chan struct{})
	var done sync.WaitGroup
	var readerr error
	var reads int
	done.Add(1)
	go func() {
		defer done.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}

			got, err := readfileretrying(dst)
			if err != nil {
				readerr = err
				return
			}
			reads++
			if !bytes.Equal(got, oldcontent) && !bytes.Equal(got, newcontent) {
				readerr = fmt.Errorf("saw a partial file of %d bytes", len(got))
				return
			}
		}
	}()

	for i := 0; i < 10; i++ {
		if err := copyfile(src, dst, 131072, true, nil); err != nil {
			t.Fatalf("copy %d failed with: %v", i, err)
		}
		if err := atomicwritefile(dst, oldcontent); err != nil {
			t.Fatal(err)
		}
	}

	close(stop)
	done.Wait()
	if readerr != nil {
		t.Errorf("after %d reads: %v", reads, readerr)
	}
	nodownloadleftovers(t, dir)
}
