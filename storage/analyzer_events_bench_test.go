package storage

import (
	"context"
	"testing"
	"time"
)

func BenchmarkStoreChangeEvent_Individual(b *testing.B) {
	tmpDir := b.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	event := ChangeEvent{
		ResourceID: "i-bench",
		ChangeType: "created",
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := storage.StoreChangeEvent(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStoreChangeEvent_Batch(b *testing.B) {
	tmpDir := b.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	batchSize := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events := make([]ChangeEvent, batchSize)
		for j := 0; j < batchSize; j++ {
			events[j] = ChangeEvent{
				ResourceID: "i-bench",
				ChangeType: "created",
				Timestamp:  time.Now(),
			}
		}
		if err := storage.StoreChangeEventBatch(ctx, events); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStoreDriftEvent_Individual(b *testing.B) {
	tmpDir := b.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	event := DriftEvent{
		ResourceID: "i-bench",
		DriftType:  "tag_drift",
		Field:      "env",
		Expected:   "prod",
		Actual:     "dev",
		Severity:   "high",
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := storage.StoreDriftEvent(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStoreDriftEvent_Batch(b *testing.B) {
	tmpDir := b.TempDir()
	storage, err := NewMVCCStorage(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	batchSize := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		events := make([]DriftEvent, batchSize)
		for j := 0; j < batchSize; j++ {
			events[j] = DriftEvent{
				ResourceID: "i-bench",
				DriftType:  "tag_drift",
				Field:      "env",
				Expected:   "prod",
				Actual:     "dev",
				Severity:   "high",
				Timestamp:  time.Now(),
			}
		}
		if err := storage.StoreDriftEventBatch(ctx, events); err != nil {
			b.Fatal(err)
		}
	}
}
