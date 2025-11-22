package jobmanager

import (
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

type memBuf struct {
	mu   sync.Mutex
	buf  []byte
	max  int
	last time.Time
}

var (
	memLogsMu sync.Mutex
	memLogs   = map[string]*memBuf{}
)

func getMem(id string) *memBuf {
	memLogsMu.Lock()
	b := memLogs[id]
	if b == nil {
		b = &memBuf{max: 1 << 20}
		memLogs[id] = b
	}
	memLogsMu.Unlock()
	return b
}

func writeMem(id string, p []byte) {
	b := getMem(id)
	b.mu.Lock()
	if len(p) > 0 {
		b.last = time.Now()
		if len(b.buf)+len(p) > b.max {
			drop := len(b.buf) + len(p) - b.max
			if drop > len(b.buf) {
				b.buf = nil
			} else {
				b.buf = b.buf[drop:]
			}
		}
		b.buf = append(b.buf, p...)
	}
	b.mu.Unlock()
}

func GetMemLogStat(id string) (int64, time.Time, bool) {
	memLogsMu.Lock()
	b := memLogs[id]
	memLogsMu.Unlock()
	if b == nil {
		return 0, time.Time{}, false
	}
	b.mu.Lock()
	sz := len(b.buf)
	lt := b.last
	b.mu.Unlock()
	if sz == 0 {
		return 0, lt, false
	}
	return int64(sz), lt, true
}

func ReadMemTail(id string, lines, bytesCount, maxBytes int) ([]byte, bool) {
	memLogsMu.Lock()
	b := memLogs[id]
	memLogsMu.Unlock()
	if b == nil {
		return nil, false
	}
	b.mu.Lock()
	data := append([]byte(nil), b.buf...)
	b.mu.Unlock()
	if len(data) == 0 {
		return nil, false
	}
	if bytesCount > 0 {
		if bytesCount > maxBytes {
			bytesCount = maxBytes
		}
		if bytesCount >= len(data) {
			return data, true
		}
		return data[len(data)-bytesCount:], true
	}
	parts := bytesSplit(data, '\n')
	if lines <= 0 || lines >= len(parts) {
		return data, true
	}
	tail := parts[len(parts)-lines-1:]
	outSize := 0
	for _, s := range tail {
		outSize += len(s)
	}
	out := make([]byte, 0, outSize+len(tail)-1)
	for i, s := range tail {
		out = append(out, s...)
		if i != len(tail)-1 {
			out = append(out, '\n')
		}
	}
	return out, true
}

func bytesSplit(b []byte, sep byte) [][]byte {
	var out [][]byte
	i := 0
	for i < len(b) {
		j := i
		for j < len(b) && b[j] != sep {
			j++
		}
		out = append(out, b[i:j])
		if j < len(b) {
			i = j + 1
		} else {
			i = j
		}
	}
	return out
}

type tsMemWriter struct {
	id      string
	targets []io.Writer
	mu      sync.Mutex
	buf     []byte
}

func NewTSMultiWriter(id string, targets ...io.Writer) io.Writer {
	return &tsMemWriter{id: id, targets: targets}
}

func (w *tsMemWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf = append(w.buf, p...)
	for {
		idx := -1
		for i, c := range w.buf {
			if c == '\n' {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:]
		prefix := time.Now().Format("2006-01-02 15:04:05")
		out := append([]byte(prefix+" "), line...)
		out = append(out, '\n')
		for _, t := range w.targets {
			_, _ = t.Write(out)
		}
		writeMem(w.id, out)
	}
	w.mu.Unlock()
	return len(p), nil
}

func buildWriters(id string, extra io.Writer) (io.Writer, io.Writer) {
	var outs []io.Writer
	var errs []io.Writer
	outs = append(outs, os.Stdout)
	errs = append(errs, os.Stderr)
	if extra != nil {
		outs = append(outs, extra)
		errs = append(errs, extra)
	}
	return NewTSMultiWriter(id, outs...), NewTSMultiWriter(id, errs...)
}

func ClearMemLog(id string) {
	memLogsMu.Lock()
	b := memLogs[id]
	if b != nil {
		b.mu.Lock()
		b.buf = nil
		b.mu.Unlock()
		delete(memLogs, id)
	}
	memLogsMu.Unlock()
}

func TrimMemLogs(maxAge time.Duration, maxTotal int) {
	memLogsMu.Lock()
	type item struct {
		id   string
		last time.Time
		size int
	}
	var items []item
	total := 0
	for id, b := range memLogs {
		b.mu.Lock()
		sz := len(b.buf)
		lt := b.last
		b.mu.Unlock()
		if sz == 0 || time.Since(lt) > maxAge {
			delete(memLogs, id)
			continue
		}
		items = append(items, item{id: id, last: lt, size: sz})
		total += sz
	}
	if total > maxTotal && len(items) > 0 {
		sort.Slice(items, func(i, j int) bool { return items[i].last.Before(items[j].last) })
		for _, it := range items {
			if total <= maxTotal {
				break
			}
			delete(memLogs, it.id)
			total -= it.size
		}
	}
	memLogsMu.Unlock()
}
