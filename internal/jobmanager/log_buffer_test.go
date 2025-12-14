package jobmanager

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestAsyncLogWriter_Memory(t *testing.T) {
	id := "test-mem"
	ClearMemLog(id)

	w := NewAsyncLogWriter(id, true, nil)
	defer w.Close()

	msg := []byte("hello world\n")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("short write")
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Read from mem
	data, ok := ReadMemTail(id, 10, 0, 1000)
	if !ok {
		t.Fatalf("mem log not found")
	}
	s := string(data)
	if !strings.Contains(s, "hello world") {
		t.Fatalf("content missing: %s", s)
	}
	// Check timestamp format (simple check)
	if !strings.Contains(s, time.Now().Format("2006-01-02")) {
		t.Fatalf("timestamp missing: %s", s)
	}
}

func TestAsyncLogWriter_File(t *testing.T) {
	id := "test-file"
	ClearMemLog(id)

	tmpFile, err := os.CreateTemp("", "rooster-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	w := NewAsyncLogWriter(id, false, tmpFile)

	msg := []byte("file log line\n")
	_, err = w.Write(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Close writer to flush
	w.Close()

	// Verify file content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "file log line") {
		t.Fatalf("file content missing: %s", s)
	}

	// Verify NO memory log
	_, ok := ReadMemTail(id, 10, 0, 1000)
	if ok {
		t.Fatalf("expected no mem log for file mode")
	}
}

func TestRingBuf_Wrap(t *testing.T) {
	rb := newRingBuf(10)
	rb.Write([]byte("12345")) // 5 bytes
	if rb.size != 5 {
		t.Fatalf("size mismatch: %d", rb.size)
	}

	rb.Write([]byte("67890")) // 5 bytes, total 10 (full)
	if rb.size != 10 {
		t.Fatalf("size mismatch: %d", rb.size)
	}

	out := rb.ReadAll()
	if string(out) != "1234567890" {
		t.Fatalf("content mismatch: %s", string(out))
	}

	rb.Write([]byte("AB")) // Overwrite "12"
	// Should be "34567890AB"
	out = rb.ReadAll()
	if string(out) != "34567890AB" {
		t.Fatalf("wrap content mismatch: %s", string(out))
	}

	// Write larger than cap
	rb.Write([]byte("XYZ1234567890")) // 13 bytes -> keep last 10: "1234567890"
	out = rb.ReadAll()
	if string(out) != "1234567890" {
		t.Fatalf("overflow content mismatch: %s", string(out))
	}
}

func TestBuildWriters_Closure(t *testing.T) {
	// Verify that buildWriters returns usable writers
	out, errw := buildWriters("test-build", nil)
	out.Write([]byte("out\n"))
	errw.Write([]byte("err\n"))

	out.Close()
	errw.Close()

	time.Sleep(100 * time.Millisecond)
	data, ok := ReadMemTail("test-build", 10, 0, 1000)
	if !ok {
		t.Fatal("log missing")
	}
	s := string(data)
	if !strings.Contains(s, "out") || !strings.Contains(s, "err") {
		t.Fatalf("missing output: %s", s)
	}
}
