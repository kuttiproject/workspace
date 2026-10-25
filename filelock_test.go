package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	lockHelperEnvDir   = "WORKSPACE_TEST_LOCK_DIR"
	lockHelperEnvIndex = "WORKSPACE_TEST_LOCK_INDEX"
	lockHelperProcs    = 4
	lockHelperRounds   = 25
)

func TestLockFileExcludes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	unlock, err := LockFile(path, 0, nil)
	if err != nil {
		t.Fatalf("first lock failed with: %v", err)
	}

	waits := 0
	start := time.Now()
	_, err = LockFile(path, 200*time.Millisecond, func() { waits++ })
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("second lock should have timed out, got: %v", err)
	}
	if time.Since(start) < 150*time.Millisecond {
		t.Error("gave up before the timeout")
	}
	if waits != 1 {
		t.Errorf("onwait was called %d times, expected once", waits)
	}

	unlock()

	unlock2, err := LockFile(path, time.Second, nil)
	if err != nil {
		t.Fatalf("lock after release failed with: %v", err)
	}
	unlock2()
}

func TestLockFileWaitsForRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	unlock, err := LockFile(path, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		unlock()
	}()

	waits := 0
	start := time.Now()
	unlock2, err := LockFile(path, 10*time.Second, func() { waits++ })
	if err != nil {
		t.Fatalf("waiting lock failed with: %v", err)
	}
	defer unlock2()

	if time.Since(start) < 250*time.Millisecond {
		t.Errorf("did not wait for the holder. Waited %v", time.Since(start))
	}
	if waits != 1 {
		t.Errorf("onwait was called %d times, expected once", waits)
	}
}

func TestLockFileUnlockIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	unlock, err := LockFile(path, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock()

	// A second holder must be able to take it, and a late repeat of the
	// first unlock must not release the second holder's lock.
	unlock2, err := LockFile(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock2()
	unlock()

	if _, err := LockFile(path, 100*time.Millisecond, nil); err == nil {
		t.Error("a stale unlock released someone else's lock")
	}
}

func TestLockFileBadPath(t *testing.T) {
	if _, err := LockFile(filepath.Join(t.TempDir(), "no", "dir", "x.lock"), 0, nil); err == nil {
		t.Error("locking in a missing directory should fail")
	}
}

func TestLockFileSerializesGoroutines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	var inside, maxinside, total int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := LockFile(path, 30*time.Second, nil)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			inside++
			total++
			if inside > maxinside {
				maxinside = inside
			}
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			unlock()
		}()
	}
	wg.Wait()

	if maxinside != 1 || total != 6 {
		t.Errorf("max holders at once: %d (want 1), total entries: %d (want 6)", maxinside, total)
	}
}

// TestLockHelperProcess is not a test. It is the body of the processes
// that TestLockFileAcrossProcesses starts.
func TestLockHelperProcess(t *testing.T) {
	dir := os.Getenv(lockHelperEnvDir)
	if dir == "" {
		t.Skip("helper process body. Not a test")
	}

	lockpath := filepath.Join(dir, "x.lock")
	counterpath := filepath.Join(dir, "counter")

	for i := 0; i < lockHelperRounds; i++ {
		unlock, err := LockFile(lockpath, time.Minute, nil)
		if err != nil {
			t.Fatalf("lock failed with: %v", err)
		}

		// A read-modify-write that loses updates without the lock.
		data, err := ReadFileRetrying(counterpath)
		if err != nil {
			unlock()
			t.Fatalf("read failed with: %v", err)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		time.Sleep(time.Millisecond)
		err = AtomicWriteFile(counterpath, []byte(strconv.Itoa(n+1)))
		unlock()
		if err != nil {
			t.Fatalf("write failed with: %v", err)
		}
	}
}

func TestLockFileAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	counterpath := filepath.Join(dir, "counter")
	if err := AtomicWriteFile(counterpath, []byte("0")); err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, lockHelperProcs)
	for i := 0; i < lockHelperProcs; i++ {
		go func(i int) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$", "-test.v")
			cmd.Env = append(os.Environ(),
				lockHelperEnvDir+"="+dir,
				lockHelperEnvIndex+"="+strconv.Itoa(i),
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				err = fmt.Errorf("helper %d failed: %v\n%s", i, err, out)
			}
			errs <- err
		}(i)
	}
	for i := 0; i < lockHelperProcs; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}

	data, _ := os.ReadFile(counterpath)
	want := lockHelperProcs * lockHelperRounds
	if got, _ := strconv.Atoi(strings.TrimSpace(string(data))); got != want {
		t.Errorf("counter is %d, expected %d. Updates were lost", got, want)
	}
	nodownloadleftovers(t, dir)
}
