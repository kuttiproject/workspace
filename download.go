package workspace

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/kuttiproject/kuttilog"
)

// Timeouts for downloads. There is deliberately no overall deadline,
// because images are large, and a slow download that keeps making
// progress is fine. They are variables so that tests can shorten them.
var (
	// How long to wait for a connection, including the TLS handshake.
	downloadConnectTimeout = 30 * time.Second

	// How long to wait for response headers after sending the request.
	downloadHeaderTimeout = 60 * time.Second

	// How long a download may go without receiving any data before it
	// is abandoned.
	downloadStallTimeout = 60 * time.Second
)

func newdownloadclient() *http.Client {
	var transport *http.Transport
	if defaulttransport, ok := http.DefaultTransport.(*http.Transport); ok {
		// Keeps proxy and HTTP/2 settings.
		transport = defaulttransport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}

	dialer := &net.Dialer{
		Timeout:   downloadConnectTimeout,
		KeepAlive: 30 * time.Second,
	}
	transport.DialContext = dialer.DialContext
	transport.TLSHandshakeTimeout = downloadConnectTimeout
	transport.ResponseHeaderTimeout = downloadHeaderTimeout

	return &http.Client{Transport: transport}
}

// stallreader restarts a timer each time data arrives.
type stallreader struct {
	io.Reader
	timer *time.Timer
}

func (sr *stallreader) Read(dst []byte) (int, error) {
	n, err := sr.Reader.Read(dst)
	if n > 0 {
		sr.timer.Reset(downloadStallTimeout)
	}

	return n, err
}

// checkdownloadlength returns an error if the server announced a length
// and a different number of bytes was received. A negative expected
// length means that the server did not announce one.
func checkdownloadlength(url string, received int64, expected int64) error {
	if expected >= 0 && received != expected {
		return fmt.Errorf(
			"download of %s was incomplete: received %d bytes, expected %d",
			url,
			received,
			expected,
		)
	}

	return nil
}

// downloadfile downloads a URL to a file. The data goes to a temporary
// file next to the destination, which is removed if anything fails, and
// which replaces the destination atomically only when the download is
// complete. A failed download never changes the destination.
func downloadfile(url string, filepath string, progress ProgressFunc) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	kuttilog.Printf(kuttilog.Debug, "Connecting to %s...", url)
	resp, err := newdownloadclient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP client returned the status: %v:%v", resp.StatusCode, resp.Status)
	}

	// From here, abandon the download if no data arrives for too long.
	var stalled atomic.Bool
	watchdog := time.AfterFunc(downloadStallTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer watchdog.Stop()

	// Same permissions as os.Create. os.CreateTemp would make 0600.
	tmpfilepath := uniquetempname(filepath)
	out, err := os.OpenFile(tmpfilepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	fail := func(err error) error {
		out.Close()
		os.Remove(tmpfilepath)
		return err
	}

	var respreader io.Reader = &stallreader{resp.Body, watchdog}
	if progress != nil {
		respreader = &progressreader{
			respreader,
			0,
			resp.ContentLength,
			progress,
		}
	}

	written, err := io.Copy(out, respreader)
	if err != nil {
		if stalled.Load() {
			err = fmt.Errorf(
				"download of %s stalled: no data received for %v",
				url,
				downloadStallTimeout,
			)
		}
		return fail(err)
	}

	err = checkdownloadlength(url, written, resp.ContentLength)
	if err != nil {
		return fail(err)
	}

	// Data must be on disk before it replaces anything.
	err = out.Sync()
	if err != nil {
		return fail(err)
	}

	err = out.Close()
	if err != nil {
		os.Remove(tmpfilepath)
		return err
	}

	kuttilog.Printf(kuttilog.Debug, "Saved to temporary file %v.", tmpfilepath)

	err = renamereplace(tmpfilepath, filepath)
	if err != nil {
		os.Remove(tmpfilepath)
		return err
	}

	kuttilog.Printf(kuttilog.Debug, "Downloaded to file %v.", filepath)

	return nil
}
