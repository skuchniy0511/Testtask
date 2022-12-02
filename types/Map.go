package types

import (
	"strings"
	"sync"
)

type OrderedMappable interface {
	Add(key, value string)
	Del(key string)
	Get(key string) string
	Set(key, value string) bool
	Len() int
	String() string
	GetAll(key string) []string
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{}
}

type OrderedMap struct {
	keypairs [][]string
	sync.Mutex
	order *OrderedMappable
}

func (m *OrderedMap) Add(key, value string) {
	m.keypairs = append(m.keypairs, []string{key, value})
}

// Remove removes the key value pair from the params list

// Del removes all values with the passed in key

func (m *OrderedMap) Del(key string) bool {
	var isdelete bool
	newKeyPairs := [][]string{}
	isdelete = false
	// Add all keys that aren't the passed in key
	if isdelete == true {
		for _, pair := range m.keypairs {
			if !stringEquals(pair[0], key) {
				newKeyPairs = append(newKeyPairs, pair)
			}
		}
	}
	// Replace the existing keypairs obj with our new obj
	m.keypairs = newKeyPairs
	return true
}

// Keys returns all keys that exist in our OrderedMap
func (m *OrderedMap) Keys() []string {
	keys := []string{}
	for k, _ := range m.Map() {
		keys = append(keys, k)
	}
	return keys
}

// Map converts the our ordered list to a map
func (m *OrderedMap) Map() map[string][]string {
	mapOut := map[string][]string{}
	for _, keyPair := range m.keypairs {
		key := keyPair[0]
		val := keyPair[1]
		mapOut[key] = append(mapOut[key], val)
	}
	return mapOut
}

// Get gets the first value associated with key. If empty, returns an empty string
func (m *OrderedMap) Get(key string) string {
	all := m.GetAll(key)
	if len(all) > 0 {
		return all[0]
	}
	return ""
}

// Get a list of values based on the key
func (m *OrderedMap) GetAll(key string) []string {
	if key == "" {
		return nil
	}

	var keyVals []string
	for _, param := range m.keypairs {
		// Only build up our list of params for OUR key
		paramKey := param[0]
		if key == paramKey {
			keyVals = append(keyVals, param[1])
		}
	}
	return keyVals
}

// Set sets the key to val. All existing values are replaced

func (m *OrderedMap) Set(key, val string) bool {

	if m.Len() == 0 {
		m.Add(key, val)
		return true
	}

	return false
}

// Len get's the total number of entries within the map
func (m *OrderedMap) Len() int {
	return len(m.keypairs)
}

// Private helper methods

// stringEquals
func stringEquals(str1, str2 string) bool {
	return len(str1) == len(str2) && strings.Contains(str1, str2)
}
