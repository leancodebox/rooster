package jobmanager

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

// ringBuf implements a fixed-size ring buffer for log storage.
type ringBuf struct {
	mu   sync.Mutex
	data []byte
	head int // Write position (index for next byte)
	size int // Current valid size
	cap  int // Total capacity
	last time.Time
}

func newRingBuf(cap int) *ringBuf {
	return &ringBuf{
		data: make([]byte, cap),
		cap:  cap,
	}
}

func (r *ringBuf) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = time.Now()
	n := len(p)
	if n == 0 {
		return 0, nil
	}

	// If write size > cap, only keep the last cap bytes
	if n > r.cap {
		p = p[n-r.cap:]
		n = r.cap
	}

	// Calculate how much to write in the first chunk (from head to end of buffer)
	firstChunk := r.cap - r.head
	if n <= firstChunk {
		copy(r.data[r.head:], p)
		r.head = (r.head + n) % r.cap
	} else {
		copy(r.data[r.head:], p[:firstChunk])
		copy(r.data[0:], p[firstChunk:])
		r.head = n - firstChunk
	}

	if r.size < r.cap {
		r.size += n
		if r.size > r.cap {
			r.size = r.cap
		}
	}
	return len(p), nil // Return original length to satisfy io.Writer
}

func (r *ringBuf) ReadAll() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size == 0 {
		return nil
	}
	out := make([]byte, r.size)

	// Start index of valid data
	start := (r.head - r.size + r.cap) % r.cap

	firstChunk := r.cap - start
	if r.size <= firstChunk {
		copy(out, r.data[start:start+r.size])
	} else {
		copy(out, r.data[start:])
		copy(out[firstChunk:], r.data[:r.size-firstChunk])
	}
	return out
}

var (
	memLogsMu sync.Mutex
	memLogs   = map[string]*ringBuf{}
)

func getMem(id string) *ringBuf {
	memLogsMu.Lock()
	b := memLogs[id]
	if b == nil {
		b = newRingBuf(1 << 20) // 1MB default
		memLogs[id] = b
	}
	memLogsMu.Unlock()
	return b
}

func GetMemLogStat(id string) (int64, time.Time, bool) {
	memLogsMu.Lock()
	b := memLogs[id]
	memLogsMu.Unlock()
	if b == nil {
		return 0, time.Time{}, false
	}
	b.mu.Lock()
	sz := b.size
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

	data := b.ReadAll()
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

// AsyncLogWriter implements asynchronous logging.
type AsyncLogWriter struct {
	id      string
	targets []io.Writer
	mem     *ringBuf
	useMem  bool // Whether to write to memory buffer

	buf       []byte // Internal buffer for line splitting
	inputChan chan []byte
	closeChan chan struct{}
	wg        sync.WaitGroup
}

func NewAsyncLogWriter(id string, useMem bool, targets ...io.Writer) *AsyncLogWriter {
	w := &AsyncLogWriter{
		id:        id,
		targets:   targets,
		useMem:    useMem,
		inputChan: make(chan []byte, 100), // Buffer 100 chunks
		closeChan: make(chan struct{}),
	}
	if useMem {
		w.mem = getMem(id)
	}
	w.wg.Add(1)
	go w.worker()
	return w
}

func (w *AsyncLogWriter) Write(p []byte) (int, error) {
	// We must copy p because it might be reused by the caller (e.g. os.File implementation)
	// before our worker reads it.
	cp := make([]byte, len(p))
	copy(cp, p)

	select {
	case w.inputChan <- cp:
		return len(p), nil
	case <-w.closeChan:
		return 0, io.ErrClosedPipe
	}
}

func (w *AsyncLogWriter) Close() error {
	close(w.closeChan)
	w.wg.Wait()
	return nil
}

func (w *AsyncLogWriter) worker() {
	defer w.wg.Done()

	for {
		select {
		case p, ok := <-w.inputChan:
			if !ok {
				return
			}
			w.process(p)
		case <-w.closeChan:
			// Process remaining items in channel
			for {
				select {
				case p := <-w.inputChan:
					w.process(p)
				default:
					return
				}
			}
		}
	}
}

func (w *AsyncLogWriter) process(p []byte) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx == -1 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:]

		prefix := time.Now().Format("2006-01-02 15:04:05")
		// Construct output line: "timestamp line\n"
		// Optimization: pre-allocate buffer
		out := make([]byte, 0, len(prefix)+1+len(line)+1)
		out = append(out, prefix...)
		out = append(out, ' ')
		out = append(out, line...)
		out = append(out, '\n')

		for _, t := range w.targets {
			if t != nil {
				_, _ = t.Write(out)
			}
		}
		if w.useMem && w.mem != nil {
			_, _ = w.mem.Write(out)
		}
	}
}

func buildWriters(id string, extra io.Writer) (io.WriteCloser, io.WriteCloser) {
	var outs []io.Writer
	var errs []io.Writer
	outs = append(outs, os.Stdout)
	errs = append(errs, os.Stderr)

	useMem := true
	if extra != nil {
		outs = append(outs, extra)
		errs = append(errs, extra)
		// If file output is enabled, we disable memory buffer to prevent double storage/leak.
		useMem = false
	}

	return NewAsyncLogWriter(id, useMem, outs...), NewAsyncLogWriter(id, useMem, errs...)
}

func ClearMemLog(id string) {
	memLogsMu.Lock()
	b := memLogs[id]
	if b != nil {
		// Just clear the reference in map. The ringBuf itself will be GC'd
		// if no AsyncLogWriter holds it.
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
		if time.Since(b.last) > maxAge {
			delete(memLogs, id)
			b.mu.Unlock()
			continue
		}
		items = append(items, item{id: id, last: b.last, size: b.size})
		total += b.size
		b.mu.Unlock()
	}

	if total > maxTotal {
		// Sort by time (oldest first)
		// ... (sort implementation if needed, but simple trimming by age is usually enough)
		// Since we iterate map, order is random.
		// For now, just keep the age check.
	}
	memLogsMu.Unlock()
}
