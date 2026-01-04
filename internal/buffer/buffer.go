package buffer

import (
	"log"
	"time"

	"github.com/motiso/sparksai-audit-service/internal/auditlog"
	"github.com/spf13/viper"
)

var bufferInstance *Buffer

func Get(datastore auditlog.AuditLogDatastore) *Buffer {
	if bufferInstance == nil {
		bufferInstance = NewBuffer(datastore)
		return bufferInstance
	}
	return bufferInstance
}

type Buffer struct {
	logChan      chan auditlog.AuditLog
	maxSize      int
	flushInterval time.Duration
	batchSize    int
	auditLogRepo auditlog.AuditLogDatastore
}

func NewBuffer(datastore auditlog.AuditLogDatastore) *Buffer {
	maxSize := viper.GetInt("AUDIT_BUFFER_MAX_SIZE")
	if maxSize == 0 {
		maxSize = 100 // default
	}

	flushIntervalSeconds := viper.GetInt("AUDIT_BUFFER_FLUSH_INTERVAL")
	if flushIntervalSeconds == 0 {
		flushIntervalSeconds = 30 // default
	}

	batchSize := viper.GetInt("AUDIT_BUFFER_BATCH_SIZE")
	if batchSize == 0 {
		batchSize = 100 // default
	}

	b := &Buffer{
		logChan:      make(chan auditlog.AuditLog, maxSize), // Buffered channel
		maxSize:      maxSize,
		flushInterval: time.Duration(flushIntervalSeconds) * time.Second,
		batchSize:    batchSize,
		auditLogRepo: datastore,
	}

	// Start background worker
	go b.startWorker()

	return b
}

// AddLogs adds audit logs to the buffer (non-blocking, thread-safe via channel)
func (b *Buffer) AddLogs(logs []auditlog.AuditLog) int {
	addedCount := 0
	for _, logEntry := range logs {
		// Non-blocking send: if channel is full, drop the log and continue
		select {
		case b.logChan <- logEntry:
			addedCount++
		default:
			// Channel is full - log error and drop
			log.Printf("[AUDIT BUFFER WARNING] Channel full, dropping audit log entry")
		}
	}
	return addedCount
}

// startWorker runs in background, continuously consuming from channel and batching
func (b *Buffer) startWorker() {
	batch := make([]auditlog.AuditLog, 0, b.batchSize)
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case logEntry := <-b.logChan:
			// Add log to current batch
			batch = append(batch, logEntry)

			// Flush if batch size reached
			if len(batch) >= b.batchSize {
				b.flush(batch)
				batch = batch[:0] // Reset batch slice but keep capacity
			}

		case <-ticker.C:
			// Time-based flush: flush if batch has any logs
			if len(batch) > 0 {
				b.flush(batch)
				batch = batch[:0] // Reset batch slice but keep capacity
			}
		}
	}
}

// flush performs batch insert (called from worker goroutine)
func (b *Buffer) flush(logs []auditlog.AuditLog) {
	if len(logs) == 0 {
		return
	}

	// Perform batch insert
	if err := b.auditLogRepo.BatchInsertAuditLogs(logs); err != nil {
		log.Printf("[AUDIT BUFFER ERROR] Failed to flush audit logs: %v", err)
		// Optionally: re-add logs to channel or handle error differently
	} else {
		log.Printf("[AUDIT BUFFER] Flushed %d audit log entries", len(logs))
	}
}
