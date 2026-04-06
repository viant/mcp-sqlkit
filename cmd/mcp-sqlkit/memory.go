package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"
)

func startMemoryReporter(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}

	log.Printf("mcp-sqlkit memory reporter enabled interval=%s", interval)
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		var stats runtime.MemStats
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				runtime.ReadMemStats(&stats)
				log.Printf(
					"mcp-sqlkit memory time=%s goroutines=%d heap_alloc=%s heap_inuse=%s heap_idle=%s heap_released=%s stack_inuse=%s sys=%s total_alloc=%s next_gc=%s num_gc=%d last_gc=%s",
					now.Format(time.RFC3339),
					runtime.NumGoroutine(),
					formatBytes(stats.HeapAlloc),
					formatBytes(stats.HeapInuse),
					formatBytes(stats.HeapIdle),
					formatBytes(stats.HeapReleased),
					formatBytes(stats.StackInuse),
					formatBytes(stats.Sys),
					formatBytes(stats.TotalAlloc),
					formatBytes(stats.NextGC),
					stats.NumGC,
					formatLastGC(stats.LastGC),
				)
			}
		}
	}()
}

func formatLastGC(lastGC uint64) string {
	if lastGC == 0 {
		return "-"
	}
	return time.Unix(0, int64(lastGC)).Format(time.RFC3339)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}

	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
