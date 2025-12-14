package server

import (
	"io"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
)

func handleJobLogStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	jobId := c.Query("jobId")
	j, ok := getJobStatusById(jobId)
	if !ok {
		// If job not found, close stream
		return
	}
	lp, ok := getJobLogPath(j)
	if !ok {
		return
	}

	f, err := os.Open(lp)
	if err != nil {
		// If file doesn't exist, return. Client will retry.
		return
	}
	defer f.Close()

	// Determine start offset
	st, err := f.Stat()
	if err != nil {
		return
	}
	fileSize := st.Size()
	offset := int64(0)

	lastEventId := c.GetHeader("Last-Event-ID")
	if lastEventId != "" {
		// Try to resume from last offset
		if lastOffset, err := strconv.ParseInt(lastEventId, 10, 64); err == nil {
			if lastOffset >= 0 && lastOffset <= fileSize {
				offset = lastOffset
			}
		}
	} else {
		// Initial connection: send tail (last 20KB)
		initBytes := 20 * 1024
		if fileSize > int64(initBytes) {
			offset = fileSize - int64(initBytes)
		}
	}

	// Send content from offset to current end
	if offset < fileSize {
		_, _ = f.Seek(offset, io.SeekStart)
		buf := make([]byte, fileSize-offset)
		n, _ := io.ReadFull(f, buf)
		if n > 0 {
			newOffset := offset + int64(n)
			_ = sse.Encode(c.Writer, sse.Event{
				Id:    strconv.FormatInt(newOffset, 10),
				Event: "message",
				Data:  string(buf[:n]),
			})
			c.Writer.Flush() // Flush is required to send data immediately
			offset = newOffset
		}
	}

	// Watch for changes
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("fsnotify error", "err", err)
		return
	}
	defer watcher.Close()

	if err = watcher.Add(lp); err != nil {
		slog.Error("fsnotify add error", "err", err)
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-shutdownCtx.Done():
			return
		case <-c.Request.Context().Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				st, err := os.Stat(lp)
				if err != nil {
					continue
				}
				newSize := st.Size()

				if newSize < offset {
					// File truncated
					offset = 0
					_, _ = f.Seek(0, io.SeekStart)
				}

				if newSize > offset {
					readSize := newSize - offset
					// Limit read size to avoid huge allocations if file grew a lot instantly
					if readSize > 1024*1024 {
						readSize = 1024 * 1024
					}

					buf := make([]byte, readSize)
					_, _ = f.Seek(offset, io.SeekStart)
					n, err := io.ReadFull(f, buf)
					if err == nil && n > 0 {
						newOffset := offset + int64(n)
						err := sse.Encode(c.Writer, sse.Event{
							Id:    strconv.FormatInt(newOffset, 10),
							Event: "message",
							Data:  string(buf[:n]),
						})
						if err != nil {
							slog.Error("sse encode error", "err", err)
							return
						}
						c.Writer.Flush() // Flush is required
						offset = newOffset
					}
				}
			}
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				slog.Info("log file removed or renamed")
				return
			}
		case <-ticker.C:
			if c.Request.Context().Err() != nil {
				return
			}
			// Send a ping event for client watchdog
			err := sse.Encode(c.Writer, sse.Event{
				Event: "ping",
				Data:  time.Now().Format(time.RFC3339),
			})
			if err != nil {
				slog.Error("sse heartbeat error", "err", err)
				return
			}
			c.Writer.Flush()
		case err, ok := <-watcher.Errors:
			if !ok {
				slog.Info("watcher error channel closed")
				return
			}
			slog.Error("watcher error", "err", err)
		}
	}
}
