package workspace

import (
	"errors"
	"io"
	"testing"
)

// step is one scripted result of a Read call.
type step struct {
	n   int
	err error
}

// scriptedreader returns the scripted results in order, one per Read.
// Where n is not zero it pretends to have filled that many bytes.
type scriptedreader struct {
	steps []step
}

func (sr *scriptedreader) Read(dst []byte) (int, error) {
	if len(sr.steps) == 0 {
		return 0, io.EOF
	}

	s := sr.steps[0]
	sr.steps = sr.steps[1:]

	return s.n, s.err
}

type progresscall struct {
	current int64
	total   int64
}

func runprogressreader(steps []step, total int64) (reads []step, calls []progresscall) {
	pr := &progressreader{
		Reader: &scriptedreader{steps},
		total:  total,
		callback: func(current, total int64) {
			calls = append(calls, progresscall{current, total})
		},
	}

	buf := make([]byte, 1024)
	for range steps {
		n, err := pr.Read(buf)
		reads = append(reads, step{n, err})
	}

	return reads, calls
}

func TestProgressReaderReportsEachChunk(t *testing.T) {
	_, calls := runprogressreader([]step{{100, nil}, {50, nil}, {0, io.EOF}}, 150)

	expected := []progresscall{{100, 150}, {150, 150}}
	if len(calls) != len(expected) {
		t.Fatalf("got calls %v, expected %v", calls, expected)
	}
	for i := range expected {
		if calls[i] != expected[i] {
			t.Errorf("call %d was %v, expected %v", i, calls[i], expected[i])
		}
	}
}

// net/http bodies with a known length commonly return the last bytes
// together with io.EOF.
func TestProgressReaderReportsBytesWithEOF(t *testing.T) {
	_, calls := runprogressreader([]step{{100, nil}, {50, io.EOF}}, 150)

	if len(calls) != 2 {
		t.Fatalf("got calls %v, expected two", calls)
	}
	if calls[1].current != 150 {
		t.Errorf("last progress was %d, expected 150", calls[1].current)
	}
}

func TestProgressReaderReportsBytesWithOtherError(t *testing.T) {
	boom := errors.New("connection reset")
	reads, calls := runprogressreader([]step{{100, nil}, {30, boom}}, 150)

	if len(calls) != 2 || calls[1].current != 130 {
		t.Errorf("got calls %v, expected the last progress to be 130", calls)
	}
	if reads[1].n != 30 || reads[1].err != boom {
		t.Errorf("second read returned %v, expected 30 bytes and the original error", reads[1])
	}
}

func TestProgressReaderDoesNotReportEmptyReads(t *testing.T) {
	_, calls := runprogressreader([]step{{0, nil}, {0, io.EOF}}, 150)

	if len(calls) != 0 {
		t.Errorf("got calls %v, expected none", calls)
	}
}

func TestProgressReaderPassesResultsThrough(t *testing.T) {
	boom := errors.New("boom")
	in := []step{{10, nil}, {0, nil}, {5, io.EOF}, {0, boom}}
	reads, _ := runprogressreader(in, 15)

	for i := range in {
		if reads[i] != in[i] {
			t.Errorf("read %d returned %v, expected %v", i, reads[i], in[i])
		}
	}
}

func TestProgressReaderWithoutCallback(t *testing.T) {
	pr := &progressreader{Reader: &scriptedreader{[]step{{10, io.EOF}}}}

	n, err := pr.Read(make([]byte, 16))
	if n != 10 || err != io.EOF {
		t.Errorf("read returned %d, %v", n, err)
	}
	if pr.current != 10 {
		t.Errorf("current is %d, expected 10", pr.current)
	}
}

// A single byte is still progress, with or without an error.
func TestProgressReaderReportsSingleByte(t *testing.T) {
	_, calls := runprogressreader([]step{{1, nil}, {1, io.EOF}}, 2)

	expected := []progresscall{{1, 2}, {2, 2}}
	if len(calls) != len(expected) || calls[0] != expected[0] || calls[1] != expected[1] {
		t.Errorf("got calls %v, expected %v", calls, expected)
	}
}
