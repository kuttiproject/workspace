package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeread struct {
	calls     int
	failures  int
	failwith  error
	succeedas []byte
}

func (f *fakeread) read(path string) ([]byte, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, f.failwith
	}
	return f.succeedas, nil
}

func fakeretry(t *testing.T, f *fakeread, transient error) {
	t.Helper()

	oldread, oldtransient := readfilefunc, istransientfileerrorf
	oldattempts, olddelay := readretryattempts, readretrydelay
	readfilefunc = f.read
	istransientfileerrorf = func(err error) bool { return errors.Is(err, transient) }
	readretryattempts, readretrydelay = 5, time.Millisecond
	t.Cleanup(func() {
		readfilefunc, istransientfileerrorf = oldread, oldtransient
		readretryattempts, readretrydelay = oldattempts, olddelay
	})
}

func TestReadFileRetryingRecovers(t *testing.T) {
	busy := errors.New("file is busy")
	f := &fakeread{failures: 3, failwith: busy, succeedas: []byte("data")}
	fakeretry(t, f, busy)

	got, err := ReadFileRetrying("anything")
	if err != nil || string(got) != "data" {
		t.Fatalf("got %q, %v", got, err)
	}
	if f.calls != 4 {
		t.Errorf("expected 4 reads, got %d", f.calls)
	}
}

func TestReadFileRetryingGivesUp(t *testing.T) {
	busy := errors.New("file is busy")
	f := &fakeread{failures: 1000, failwith: busy}
	fakeretry(t, f, busy)

	if _, err := ReadFileRetrying("anything"); !errors.Is(err, busy) {
		t.Fatalf("expected the busy error, got %v", err)
	}
	if f.calls != 5 {
		t.Errorf("expected 5 attempts, got %d", f.calls)
	}
}

func TestReadFileRetryingDoesNotRetryOtherErrors(t *testing.T) {
	busy := errors.New("file is busy")
	other := errors.New("something else")
	f := &fakeread{failures: 1000, failwith: other}
	fakeretry(t, f, busy)

	if _, err := ReadFileRetrying("anything"); !errors.Is(err, other) {
		t.Fatalf("expected the other error, got %v", err)
	}
	if f.calls != 1 {
		t.Errorf("a non-transient error was retried %d times", f.calls-1)
	}
}

func TestReadFileRetryingMissingFile(t *testing.T) {
	start := time.Now()
	_, err := ReadFileRetrying(filepath.Join(t.TempDir(), "nothere"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not-exist, got %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Error("a missing file should fail immediately")
	}
}

func TestReadFileRetryingReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := AtomicWriteFile(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFileRetrying(path)
	if err != nil || string(got) != "hello" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestAtomicWriteFileExported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")

	for _, content := range []string{"one", "two, longer", "3"} {
		if err := AtomicWriteFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); string(got) != content {
			t.Errorf("got %q, expected %q", got, content)
		}
	}
	nodownloadleftovers(t, dir)

	if err := AtomicWriteFile(filepath.Join(dir, "no", "such", "dir", "f"), []byte("x")); err == nil {
		t.Error("writing into a missing directory should fail")
	}
}
