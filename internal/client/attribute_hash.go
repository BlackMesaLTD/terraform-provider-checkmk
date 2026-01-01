package client

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// AttributeHasher generates deterministic hashes for attribute maps.
// Uses version-specific CompareKeyFields to select relevant fields.
type AttributeHasher struct {
	types *VersionedTypes
}

// NewAttributeHasher creates a new AttributeHasher.
// Returns nil if types is nil.
func NewAttributeHasher(types *VersionedTypes) *AttributeHasher {
	if types == nil {
		return nil
	}
	return &AttributeHasher{types: types}
}

// HashHostAttributes generates a hash for host attributes.
// Uses HostCreateAttributeCompareKeyFields to select relevant fields.
func (h *AttributeHasher) HashHostAttributes(attributes map[string]interface{}) string {
	if h == nil || h.types == nil {
		return ""
	}
	compareKeys := h.types.HostCreateAttributeCompareKeyFields()
	return hashAttributesWithKeys(attributes, compareKeys)
}

// HashFolderAttributes generates a hash for folder attributes.
// Uses FolderCreateAttributeCompareKeyFields to select relevant fields.
func (h *AttributeHasher) HashFolderAttributes(attributes map[string]interface{}) string {
	if h == nil || h.types == nil {
		return ""
	}
	compareKeys := h.types.FolderCreateAttributeCompareKeyFields()
	return hashAttributesWithKeys(attributes, compareKeys)
}

// HashAttributes generates a hash using provided compare keys.
func (h *AttributeHasher) HashAttributes(attributes map[string]interface{}, compareKeys []string) string {
	return hashAttributesWithKeys(attributes, compareKeys)
}

// hashAttributesWithKeys generates a deterministic hash for attributes
// using only the specified compare keys.
func hashAttributesWithKeys(attributes map[string]interface{}, compareKeys []string) string {
	if len(attributes) == 0 || len(compareKeys) == 0 {
		return ""
	}

	// Create a map of compare keys for O(1) lookup
	keySet := make(map[string]bool, len(compareKeys))
	for _, k := range compareKeys {
		keySet[k] = true
	}

	// Filter attributes to only include compare keys
	filtered := make(map[string]interface{}, len(compareKeys))
	for key, value := range attributes {
		if keySet[key] && value != nil {
			filtered[key] = normalizeValue(value)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	sortedKeys := make([]string, 0, len(filtered))
	for k := range filtered {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Build ordered map for consistent JSON
	ordered := make([]orderedKeyValue, len(sortedKeys))
	for i, k := range sortedKeys {
		ordered[i] = orderedKeyValue{Key: k, Value: filtered[k]}
	}

	// Marshal to JSON
	data, err := json.Marshal(ordered)
	if err != nil {
		return ""
	}

	// Hash the JSON
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// orderedKeyValue represents a key-value pair for ordered JSON marshaling.
type orderedKeyValue struct {
	Key   string      `json:"k"`
	Value interface{} `json:"v"`
}

// normalizeValue normalizes a value for consistent hashing.
// Handles nested maps and slices.
func normalizeValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return normalizeMap(v)
	case []interface{}:
		return normalizeSlice(v)
	default:
		return v
	}
}

// normalizeMap recursively normalizes a map for consistent hashing.
func normalizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if v != nil {
			result[k] = normalizeValue(v)
		}
	}
	return result
}

// normalizeSlice normalizes a slice for consistent hashing.
func normalizeSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = normalizeValue(v)
	}
	return result
}
