// Package store provides the data persistence layer using BBolt.
package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/johncarpenter/trajectory-memory/internal/types"
	bolt "go.etcd.io/bbolt"
)

var (
	// ErrSessionNotFound is returned when a session doesn't exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrNoActiveSession is returned when no session is recording.
	ErrNoActiveSession = errors.New("no active session")
	// ErrSessionAlreadyActive is returned when trying to start while recording.
	ErrSessionAlreadyActive = errors.New("a session is already recording")
)

// Bucket names
var (
	bucketSessions       = []byte("sessions")
	bucketActive         = []byte("active")
	bucketIndex          = []byte("index")
	bucketStrategyUsage  = []byte("strategy_usage")
)

// Store defines the interface for session persistence.
type Store interface {
	CreateSession(s *types.Session) error
	GetSession(id string) (*types.Session, error)
	UpdateSession(s *types.Session) error
	AppendStep(sessionID string, step types.TrajectoryStep) error
	ListSessions(limit int, offset int) ([]types.SessionMetadata, error)
	SearchSessions(query string, limit int) ([]types.SessionMetadata, error)
	SetOutcome(sessionID string, outcome types.Outcome) error
	GetActiveSession() (*types.Session, error)
	SetActiveSession(sessionID string) error
	ClearActiveSession() error
	DeleteSession(id string) error
	ExportAll(w io.Writer) error
	ImportAll(r io.Reader) error
	Close() error
}

// BoltStore implements Store using BBolt.
type BoltStore struct {
	db *bolt.DB
	mu sync.RWMutex
}

// NewBoltStore creates a new BBolt-backed store.
func NewBoltStore(dbPath string) (*BoltStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketSessions, bucketActive, bucketIndex, bucketStrategyUsage} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

// CreateSession creates a new session in the store.
func (s *BoltStore) CreateSession(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		index := tx.Bucket(bucketIndex)

		// Check if session already exists
		if sessions.Get([]byte(session.ID)) != nil {
			return fmt.Errorf("session %s already exists", session.ID)
		}

		// Store the full session
		data, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		if err := sessions.Put([]byte(session.ID), data); err != nil {
			return fmt.Errorf("failed to store session: %w", err)
		}

		// Store the metadata index
		meta := session.ToMetadata()
		metaData, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := index.Put([]byte(session.ID), metaData); err != nil {
			return fmt.Errorf("failed to store metadata: %w", err)
		}

		return nil
	})
}

// GetSession retrieves a session by ID.
func (s *BoltStore) GetSession(id string) (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var session types.Session
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		data := b.Get([]byte(id))
		if data == nil {
			return ErrSessionNotFound
		}
		return json.Unmarshal(data, &session)
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateSession updates an existing session.
func (s *BoltStore) UpdateSession(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		index := tx.Bucket(bucketIndex)

		// Check if session exists
		if sessions.Get([]byte(session.ID)) == nil {
			return ErrSessionNotFound
		}

		// Update the full session
		data, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		if err := sessions.Put([]byte(session.ID), data); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Update the metadata index
		meta := session.ToMetadata()
		metaData, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := index.Put([]byte(session.ID), metaData); err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}

		return nil
	})
}

// AppendStep adds a step to an existing session.
func (s *BoltStore) AppendStep(sessionID string, step types.TrajectoryStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		index := tx.Bucket(bucketIndex)

		// Get existing session
		data := sessions.Get([]byte(sessionID))
		if data == nil {
			return ErrSessionNotFound
		}

		var session types.Session
		if err := json.Unmarshal(data, &session); err != nil {
			return fmt.Errorf("failed to unmarshal session: %w", err)
		}

		// Truncate input/output summaries
		step.InputSummary = types.TruncateString(step.InputSummary, types.MaxInputSummaryLen)
		step.OutputSummary = types.TruncateString(step.OutputSummary, types.MaxOutputSummaryLen)

		// Append step
		session.Steps = append(session.Steps, step)

		// Save updated session
		updatedData, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		if err := sessions.Put([]byte(sessionID), updatedData); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Update metadata (step count changed)
		meta := session.ToMetadata()
		metaData, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := index.Put([]byte(sessionID), metaData); err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}

		return nil
	})
}

// ListSessions returns sessions ordered by most recent first.
func (s *BoltStore) ListSessions(limit int, offset int) ([]types.SessionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []types.SessionMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketIndex)
		c := b.Cursor()

		// Collect all metadata first (ULIDs are sortable, so reverse order = most recent)
		var all []types.SessionMetadata
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var meta types.SessionMetadata
			if err := json.Unmarshal(v, &meta); err != nil {
				continue // Skip malformed entries
			}
			all = append(all, meta)
		}

		// Apply offset and limit
		if offset >= len(all) {
			return nil
		}
		end := offset + limit
		if end > len(all) {
			end = len(all)
		}
		results = all[offset:end]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// SearchSessions searches sessions by keyword in task_prompt, summary, and tags.
func (s *BoltStore) SearchSessions(query string, limit int) ([]types.SessionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(query)
	var results []types.SessionMetadata

	err := s.db.View(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		c := sessions.Cursor()

		// Search through all sessions (reverse order for most recent first)
		for k, v := c.Last(); k != nil && len(results) < limit; k, v = c.Prev() {
			var session types.Session
			if err := json.Unmarshal(v, &session); err != nil {
				continue
			}

			// Case-insensitive search across multiple fields
			if s.matchesQuery(&session, query) {
				results = append(results, session.ToMetadata())
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// matchesQuery checks if a session matches the search query.
func (s *BoltStore) matchesQuery(session *types.Session, query string) bool {
	// Check task prompt
	if strings.Contains(strings.ToLower(session.TaskPrompt), query) {
		return true
	}
	// Check summary
	if strings.Contains(strings.ToLower(session.Summary), query) {
		return true
	}
	// Check tags
	for _, tag := range session.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

// SetOutcome sets or updates the outcome for a session.
func (s *BoltStore) SetOutcome(sessionID string, outcome types.Outcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		index := tx.Bucket(bucketIndex)

		// Get existing session
		data := sessions.Get([]byte(sessionID))
		if data == nil {
			return ErrSessionNotFound
		}

		var session types.Session
		if err := json.Unmarshal(data, &session); err != nil {
			return fmt.Errorf("failed to unmarshal session: %w", err)
		}

		// Set outcome and update status
		session.Outcome = &outcome
		session.Status = types.StatusScored

		// Save updated session
		updatedData, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		if err := sessions.Put([]byte(sessionID), updatedData); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}

		// Update metadata
		meta := session.ToMetadata()
		metaData, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		if err := index.Put([]byte(sessionID), metaData); err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}

		return nil
	})
}

// GetActiveSession returns the currently recording session, if any.
func (s *BoltStore) GetActiveSession() (*types.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var session *types.Session
	err := s.db.View(func(tx *bolt.Tx) error {
		active := tx.Bucket(bucketActive)
		sessionID := active.Get([]byte("current"))
		if sessionID == nil || len(sessionID) == 0 {
			return ErrNoActiveSession
		}

		sessions := tx.Bucket(bucketSessions)
		data := sessions.Get(sessionID)
		if data == nil {
			return ErrNoActiveSession
		}

		session = &types.Session{}
		return json.Unmarshal(data, session)
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// SetActiveSession sets the active recording session.
func (s *BoltStore) SetActiveSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		active := tx.Bucket(bucketActive)
		sessions := tx.Bucket(bucketSessions)

		// Check if session exists
		if sessions.Get([]byte(sessionID)) == nil {
			return ErrSessionNotFound
		}

		// Check if there's already an active session
		current := active.Get([]byte("current"))
		if current != nil && len(current) > 0 && string(current) != sessionID {
			return ErrSessionAlreadyActive
		}

		return active.Put([]byte("current"), []byte(sessionID))
	})
}

// ClearActiveSession clears the active session marker.
func (s *BoltStore) ClearActiveSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		active := tx.Bucket(bucketActive)
		return active.Delete([]byte("current"))
	})
}

// DeleteSession removes a session from the store.
func (s *BoltStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		sessions := tx.Bucket(bucketSessions)
		index := tx.Bucket(bucketIndex)
		active := tx.Bucket(bucketActive)

		// Check if session exists
		if sessions.Get([]byte(id)) == nil {
			return ErrSessionNotFound
		}

		// Clear active if this is the active session
		current := active.Get([]byte("current"))
		if current != nil && string(current) == id {
			if err := active.Delete([]byte("current")); err != nil {
				return err
			}
		}

		// Delete from both buckets
		if err := sessions.Delete([]byte(id)); err != nil {
			return err
		}
		return index.Delete([]byte(id))
	})
}

// ExportAll writes all sessions as JSONL to the writer.
func (s *BoltStore) ExportAll(w io.Writer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSessions)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var session types.Session
			if err := json.Unmarshal(v, &session); err != nil {
				continue
			}
			line, err := json.Marshal(session)
			if err != nil {
				continue
			}
			if _, err := w.Write(append(line, '\n')); err != nil {
				return err
			}
		}
		return nil
	})
}

// ImportAll reads sessions from JSONL and imports them.
func (s *BoltStore) ImportAll(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var session types.Session
		if err := json.Unmarshal(scanner.Bytes(), &session); err != nil {
			continue // Skip malformed lines
		}
		// Use CreateSession which handles duplicates
		if err := s.CreateSession(&session); err != nil {
			// If session exists, try updating it
			if strings.Contains(err.Error(), "already exists") {
				if err := s.UpdateSession(&session); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return scanner.Err()
}

// Close closes the database connection.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// NewULID generates a new ULID-like ID.
// RecordStrategyUsage records which strategy was used for a session.
func (s *BoltStore) RecordStrategyUsage(usage types.StrategyUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketStrategyUsage)

		data, err := json.Marshal(usage)
		if err != nil {
			return fmt.Errorf("failed to marshal strategy usage: %w", err)
		}

		// Key format: tag:session_id
		key := fmt.Sprintf("%s:%s", usage.Tag, usage.SessionID)
		return bucket.Put([]byte(key), data)
	})
}

// GetStrategyUsage retrieves strategy usage records for a tag.
func (s *BoltStore) GetStrategyUsage(tag string, limit int) ([]types.StrategyUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var usages []types.StrategyUsage
	prefix := []byte(tag + ":")

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketStrategyUsage)
		c := bucket.Cursor()

		count := 0
		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), tag+":"); k, v = c.Next() {
			if limit > 0 && count >= limit {
				break
			}

			var usage types.StrategyUsage
			if err := json.Unmarshal(v, &usage); err != nil {
				continue // Skip malformed entries
			}
			usages = append(usages, usage)
			count++
		}
		return nil
	})

	return usages, err
}

// GetStrategyStats calculates aggregate statistics for strategies under a tag.
func (s *BoltStore) GetStrategyStats(tag string) (map[string]*types.Strategy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]*types.Strategy)
	prefix := []byte(tag + ":")

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketStrategyUsage)
		c := bucket.Cursor()

		// Accumulate scores and counts per strategy
		scoreSums := make(map[string]float64)
		scoreCounts := make(map[string]int)

		for k, v := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), tag+":"); k, v = c.Next() {
			var usage types.StrategyUsage
			if err := json.Unmarshal(v, &usage); err != nil {
				continue
			}

			if _, ok := stats[usage.StrategyName]; !ok {
				stats[usage.StrategyName] = &types.Strategy{
					Name: usage.StrategyName,
				}
			}

			if usage.Score > 0 {
				scoreSums[usage.StrategyName] += usage.Score
				scoreCounts[usage.StrategyName]++
			}
			stats[usage.StrategyName].SessionCount++
		}

		// Calculate averages
		for name, strat := range stats {
			if count := scoreCounts[name]; count > 0 {
				strat.AvgScore = scoreSums[name] / float64(count)
			}
		}

		return nil
	})

	return stats, err
}

// UpdateStrategyUsageScore updates the score for a strategy usage record.
func (s *BoltStore) UpdateStrategyUsageScore(sessionID string, score float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketStrategyUsage)
		c := bucket.Cursor()

		// Find the usage record for this session
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var usage types.StrategyUsage
			if err := json.Unmarshal(v, &usage); err != nil {
				continue
			}

			if usage.SessionID == sessionID {
				usage.Score = score
				data, err := json.Marshal(usage)
				if err != nil {
					return err
				}
				return bucket.Put(k, data)
			}
		}

		return nil // No usage found for this session, that's OK
	})
}

// Using a simple implementation to avoid extra dependencies.
func NewULID() string {
	t := time.Now().UTC()
	timestamp := uint64(t.UnixMilli())

	// Encode timestamp (10 chars in base32)
	const encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var ts [10]byte
	for i := 9; i >= 0; i-- {
		ts[i] = encoding[timestamp&31]
		timestamp >>= 5
	}

	// Generate random part (16 chars)
	var rnd [16]byte
	for i := range rnd {
		rnd[i] = encoding[rand.Intn(32)]
	}

	return string(ts[:]) + string(rnd[:])
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
