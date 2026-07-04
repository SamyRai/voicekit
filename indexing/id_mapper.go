package indexing

import "sync"

// IDMapper handles bidirectional mapping between string IDs and integer IDs
// HNSW libraries typically require integer IDs for efficiency
type IDMapper struct {
	stringToInt map[string]int
	intToString map[int]string
	nextID      int
	mu          sync.RWMutex
}

// NewIDMapper creates a new ID mapper
func NewIDMapper() *IDMapper {
	return &IDMapper{
		stringToInt: make(map[string]int),
		intToString: make(map[int]string),
		nextID:      1, // Start from 1 to avoid 0 conflicts
	}
}

// GetIntID returns the integer ID for a string ID, creating one if it doesn't exist
func (m *IDMapper) GetIntID(stringID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, exists := m.stringToInt[stringID]; exists {
		return id
	}

	// Create new ID
	id := m.nextID
	m.nextID++

	m.stringToInt[stringID] = id
	m.intToString[id] = stringID

	return id
}

// LookupIntID returns the integer ID for a string ID without creating a mapping.
func (m *IDMapper) LookupIntID(stringID string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, exists := m.stringToInt[stringID]
	return id, exists
}

// GetStringID returns the string ID for an integer ID
func (m *IDMapper) GetStringID(intID int) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stringID, exists := m.intToString[intID]
	return stringID, exists
}

// RemoveID removes an ID mapping
func (m *IDMapper) RemoveID(stringID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, exists := m.stringToInt[stringID]; exists {
		delete(m.stringToInt, stringID)
		delete(m.intToString, id)
	}
}

// Size returns the number of mapped IDs
func (m *IDMapper) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.stringToInt)
}

// Clear removes all mappings
func (m *IDMapper) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stringToInt = make(map[string]int)
	m.intToString = make(map[int]string)
	m.nextID = 1
}
