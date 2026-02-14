package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/types"
	bolt "go.etcd.io/bbolt"
)

var (
	// ErrOptimizationNotFound is returned when an optimization record doesn't exist.
	ErrOptimizationNotFound = errors.New("optimization record not found")
)

// Optimization bucket names
var (
	bucketOptimizations   = []byte("optimizations")
	bucketCuratedExamples = []byte("curated_examples")
	bucketTriggerConfig   = []byte("trigger_config")
)

// OptimizationStore defines the interface for optimization persistence.
type OptimizationStore interface {
	// Optimization records
	CreateOptimization(r *types.OptimizationRecord) error
	GetOptimization(id string) (*types.OptimizationRecord, error)
	UpdateOptimization(r *types.OptimizationRecord) error
	ListOptimizations(filePath string, tag string, limit int) ([]types.OptimizationRecord, error)
	GetLatestOptimization(tag string) (*types.OptimizationRecord, error)

	// Curated examples
	SaveCuratedExamples(tag string, examples []types.CuratedExample) error
	GetCuratedExamples(tag string) ([]types.CuratedExample, error)

	// Trigger config
	GetTriggerConfig() (*TriggerConfig, error)
	SaveTriggerConfig(config *TriggerConfig) error
}

// TriggerConfig configures auto-optimization triggers.
type TriggerConfig struct {
	SessionThreshold int      `json:"session_threshold"`
	MinScoreGap      float64  `json:"min_score_gap"`
	Enabled          bool     `json:"enabled"`
	WatchFiles       []string `json:"watch_files"`
}

// DefaultTriggerConfig returns the default trigger configuration.
func DefaultTriggerConfig() *TriggerConfig {
	return &TriggerConfig{
		SessionThreshold: 10,
		MinScoreGap:      0.05,
		Enabled:          false,
		WatchFiles:       []string{},
	}
}

// EnsureOptimizationBuckets creates the optimization-related buckets if they don't exist.
func (s *BoltStore) EnsureOptimizationBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketOptimizations, bucketCuratedExamples, bucketTriggerConfig} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
}

// CreateOptimization creates a new optimization record.
func (s *BoltStore) CreateOptimization(r *types.OptimizationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketOptimizations)

		if r.ID == "" {
			r.ID = NewULID()
		}
		r.CreatedAt = time.Now()

		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("failed to marshal optimization: %w", err)
		}

		return b.Put([]byte(r.ID), data)
	})
}

// GetOptimization retrieves an optimization record by ID.
func (s *BoltStore) GetOptimization(id string) (*types.OptimizationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return nil, err
	}

	var record types.OptimizationRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketOptimizations)
		data := b.Get([]byte(id))
		if data == nil {
			return ErrOptimizationNotFound
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// UpdateOptimization updates an existing optimization record.
func (s *BoltStore) UpdateOptimization(r *types.OptimizationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketOptimizations)
		if b.Get([]byte(r.ID)) == nil {
			return ErrOptimizationNotFound
		}

		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("failed to marshal optimization: %w", err)
		}

		return b.Put([]byte(r.ID), data)
	})
}

// ListOptimizations lists optimization records, optionally filtered by file and/or tag.
func (s *BoltStore) ListOptimizations(filePath string, tag string, limit int) ([]types.OptimizationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return nil, err
	}

	var results []types.OptimizationRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketOptimizations)
		c := b.Cursor()

		// Iterate in reverse order (most recent first, since ULIDs sort chronologically)
		for k, v := c.Last(); k != nil && len(results) < limit; k, v = c.Prev() {
			var record types.OptimizationRecord
			if err := json.Unmarshal(v, &record); err != nil {
				continue
			}

			// Apply filters
			if filePath != "" && record.TargetFile != filePath {
				continue
			}
			if tag != "" && record.Tag != tag {
				continue
			}

			results = append(results, record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// GetLatestOptimization gets the most recent optimization record for a tag.
func (s *BoltStore) GetLatestOptimization(tag string) (*types.OptimizationRecord, error) {
	records, err := s.ListOptimizations("", tag, 1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

// SaveCuratedExamples saves curated examples for a tag.
func (s *BoltStore) SaveCuratedExamples(tag string, examples []types.CuratedExample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCuratedExamples)

		data, err := json.Marshal(examples)
		if err != nil {
			return fmt.Errorf("failed to marshal examples: %w", err)
		}

		return b.Put([]byte(tag), data)
	})
}

// GetCuratedExamples retrieves curated examples for a tag.
func (s *BoltStore) GetCuratedExamples(tag string) ([]types.CuratedExample, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return nil, err
	}

	var examples []types.CuratedExample
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCuratedExamples)
		data := b.Get([]byte(tag))
		if data == nil {
			return nil // No examples yet, return empty
		}
		return json.Unmarshal(data, &examples)
	})
	return examples, err
}

// GetTriggerConfig retrieves the trigger configuration.
func (s *BoltStore) GetTriggerConfig() (*TriggerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return nil, err
	}

	var config TriggerConfig
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTriggerConfig)
		data := b.Get([]byte("config"))
		if data == nil {
			// Return default config
			config = *DefaultTriggerConfig()
			return nil
		}
		return json.Unmarshal(data, &config)
	})
	return &config, err
}

// SaveTriggerConfig saves the trigger configuration.
func (s *BoltStore) SaveTriggerConfig(config *TriggerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure buckets exist
	if err := s.EnsureOptimizationBuckets(); err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketTriggerConfig)

		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		return b.Put([]byte("config"), data)
	})
}
