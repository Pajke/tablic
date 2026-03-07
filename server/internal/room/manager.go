package room

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"tablic/server/internal/storage"
)

// Manager holds all active game rooms.
type Manager struct {
	mu      sync.RWMutex
	rooms   map[string]*Room
	storage *storage.Storage
}

func NewManager(st *storage.Storage) *Manager {
	return &Manager{rooms: make(map[string]*Room), storage: st}
}

// Storage returns the shared storage instance (may be nil).
func (m *Manager) Storage() *storage.Storage { return m.storage }

// Create creates a new room and returns it.
func (m *Manager) Create(maxPlayers int) *Room {
	id := generateID()
	r := newRoom(id, maxPlayers, m.storage)
	m.mu.Lock()
	m.rooms[id] = r
	m.mu.Unlock()
	return r
}

// Get retrieves a room by ID.
func (m *Manager) Get(id string) (*Room, error) {
	m.mu.RLock()
	r, ok := m.rooms[id]
	m.mu.RUnlock()
	if !ok {
		return nil, errors.New("room not found: " + id)
	}
	return r, nil
}

// Remove deletes a room.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	delete(m.rooms, id)
	m.mu.Unlock()
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
