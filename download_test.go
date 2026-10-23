package workspace

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func payload(size int, seed byte) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = seed + byte(i%251)
	}
	return data
}

func shorttimeouts(t *testing.T, connect, header, stall time.Duration) {
	t.Helper()

	oldc, oldh, olds := downloadConnectTimeout, downloadHeaderTimeout, downloadStallTimeout
	downloadConnectTimeout, downloadHeaderTimeout, downloadStallTimeout = connect, header, stall
	t.Cleanup(func() {
		downloadConnectTimeout, downloadHeaderTimeout, downloadStallTimeout = oldc, oldh, olds
	})
}

func nodownloadleftovers(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") || strings.HasSuffix(entry.Name(), ".download") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

func contentserver(t *testing.T, content []byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		w.Write(content)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDownloadReplacesFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	content := payload(3<<20, 1)
	server := contentserver(t, content)

	if err := os.WriteFile(dest, []byte("old"), 0o666); err != nil {
		t.Fatal(err)
	}

	if err := downloadfile(server.URL, dest, nil); err != nil {
		t.Fatalf("download failed with: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if !bytes.Equal(got, content) {
		t.Errorf("destination holds %d bytes, expected the %d downloaded bytes", len(got), len(content))
	}
	nodownloadleftovers(t, dir)

	// The file must get the same permissions as a file made with os.Create,
	// which a temporary file made with os.CreateTemp would not.
	ref, err := os.Create(filepath.Join(dir, "reference"))
	if err != nil {
		t.Fatal(err)
	}
	ref.Close()
	os.Remove(dest)
	if err := downloadfile(server.URL, dest, nil); err != nil {
		t.Fatal(err)
	}
	refinfo, _ := os.Stat(filepath.Join(dir, "reference"))
	destinfo, _ := os.Stat(dest)
	if refinfo.Mode().Perm() != destinfo.Mode().Perm() {
		t.Errorf("downloaded file has mode %v, os.Create gives %v", destinfo.Mode().Perm(), refinfo.Mode().Perm())
	}
}

func TestDownloadReportsProgress(t *testing.T) {
	dir := t.TempDir()
	server := contentserver(t, payload(200000, 2))

	var calls int
	var lasttotal int64
	err := downloadfile(server.URL, filepath.Join(dir, "p.bin"), func(current, total int64) {
		calls++
		lasttotal = total
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 || lasttotal != 200000 {
		t.Errorf("progress called %d times, last total %d", calls, lasttotal)
	}
}

func TestDownloadBadStatusKeepsFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	os.WriteFile(dest, []byte("old"), 0o666)

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	if err := downloadfile(server.URL, dest, nil); err == nil {
		t.Fatal("a 404 should have failed")
	}

	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Errorf("destination changed to %q", got)
	}
	nodownloadleftovers(t, dir)
}

// The server promises more data than it sends, then closes the connection.
func TestDownloadTruncatedKeepsFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	os.WriteFile(dest, []byte("old"), 0o666)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		buf := make([]byte, 4096)
		conn.Read(buf)
		fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: 1000000\r\n\r\n")
		conn.Write(payload(1000, 3))
		conn.Close()
	}()

	err = downloadfile("http://"+listener.Addr().String(), dest, nil)
	if err == nil {
		t.Fatal("a truncated download should have failed")
	}

	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Errorf("destination changed to %d bytes", len(got))
	}
	nodownloadleftovers(t, dir)
}

func TestDownloadLengthCheck(t *testing.T) {
	if err := checkdownloadlength("u", 10, 10); err != nil {
		t.Errorf("equal lengths rejected: %v", err)
	}
	if err := checkdownloadlength("u", 10, -1); err != nil {
		t.Errorf("unknown length rejected: %v", err)
	}
	if err := checkdownloadlength("u", 9, 10); err == nil {
		t.Error("short download accepted")
	}
	if err := checkdownloadlength("u", 11, 10); err == nil {
		t.Error("long download accepted")
	}
}

// Some data arrives, then nothing.
func TestDownloadStallKeepsFile(t *testing.T) {
	shorttimeouts(t, time.Second, time.Second, 300*time.Millisecond)

	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	os.WriteFile(dest, []byte("old"), 0o666)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload(1000, 4))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer server.Close()

	start := time.Now()
	err := downloadfile(server.URL, dest, nil)
	if err == nil {
		t.Fatal("a stalled download should have failed")
	}
	if !strings.Contains(err.Error(), "stalled") {
		t.Errorf("error should say the download stalled. Got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v to give up on a stalled download", elapsed)
	}

	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Errorf("destination changed to %d bytes", len(got))
	}
	nodownloadleftovers(t, dir)
}

// A slow download that keeps making progress must not be abandoned.
func TestDownloadSlowButSteadySucceeds(t *testing.T) {
	shorttimeouts(t, time.Second, time.Second, 400*time.Millisecond)

	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 6; i++ {
			w.Write(payload(100, byte(i)))
			w.(http.Flusher).Flush()
			time.Sleep(150 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Total time is about 900 ms: more than the stall timeout.
	if err := downloadfile(server.URL, dest, nil); err != nil {
		t.Fatalf("a steady download failed with: %v", err)
	}
	if got, _ := os.ReadFile(dest); len(got) != 600 {
		t.Errorf("got %d bytes, expected 600", len(got))
	}
}

func TestDownloadHeaderTimeout(t *testing.T) {
	shorttimeouts(t, time.Second, 300*time.Millisecond, time.Second)

	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer server.Close()

	start := time.Now()
	if err := downloadfile(server.URL, filepath.Join(dir, "x.bin"), nil); err == nil {
		t.Fatal("a server that never answers should have caused a failure")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v to give up", elapsed)
	}
	nodownloadleftovers(t, dir)
}

func TestDownloadConcurrentToSameFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	content := payload(2<<20, 5)
	server := contentserver(t, content)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- downloadfile(server.URL, dest, nil)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("a concurrent download failed with: %v", err)
		}
	}

	if got, _ := os.ReadFile(dest); !bytes.Equal(got, content) {
		t.Errorf("destination is not the expected content (%d bytes)", len(got))
	}
	nodownloadleftovers(t, dir)
}

// While a download replaces a file, readers see the old file or the new
// one in full. They never find it missing or partial.
func TestDownloadReplacementIsAtomic(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "image.bin")
	oldcontent := payload(1<<20, 6)
	newcontent := payload(2<<20, 7)
	if err := os.WriteFile(dest, oldcontent, 0o666); err != nil {
		t.Fatal(err)
	}
	server := contentserver(t, newcontent)

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

			got, err := readfileretrying(dest)
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

	for i := 0; i < 15; i++ {
		if err := downloadfile(server.URL, dest, nil); err != nil {
			t.Fatalf("download %d failed with: %v", i, err)
		}
		// Put the old content back for the next round. Atomically,
		// as the code under test would.
		if err := atomicwritefile(dest, oldcontent); err != nil {
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

func TestUniqueTempNames(t *testing.T) {
	// A frozen, coarse clock must not cause duplicate names.
	oldnow := tempnamenow
	tempnamenow = func() int64 { return 1234567890 }
	t.Cleanup(func() { tempnamenow = oldnow })

	const goroutines, each = 8, 5000
	names := make(chan string, goroutines*each)
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				names <- uniquetempname("file")
			}
		}()
	}
	wg.Wait()
	close(names)

	seen := map[string]bool{}
	for name := range names {
		if seen[name] {
			t.Fatalf("duplicate temporary file name: %s", name)
		}
		seen[name] = true
	}
}
