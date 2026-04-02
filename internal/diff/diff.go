// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// DefaultIgnoreFields are fields ignored by default when computing diffs.
var DefaultIgnoreFields = []string{
	".metadata.resourceVersion",
	".metadata.managedFields",
	".metadata.generation",
	".metadata.uid",
	".metadata.creationTimestamp",
	".metadata.annotations.kubectl\\.kubernetes\\.io/last-applied-configuration",
	".status.observedGeneration",
}

// ComputeMergePatch computes a JSON merge patch (RFC 7386) between old and new JSON.
func ComputeMergePatch(oldJSON, newJSON []byte) ([]byte, error) {
	patch, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		return nil, fmt.Errorf("computing merge patch: %w", err)
	}
	return patch, nil
}

// ApplyMergePatch applies a JSON merge patch to a document.
func ApplyMergePatch(doc, patch []byte) ([]byte, error) {
	result, err := jsonpatch.MergePatch(doc, patch)
	if err != nil {
		return nil, fmt.Errorf("applying merge patch: %w", err)
	}
	return result, nil
}

// ApplyMergePatches applies a series of merge patches to a base document sequentially.
func ApplyMergePatches(base []byte, patches [][]byte) ([]byte, error) {
	current := base
	for i, p := range patches {
		var err error
		current, err = ApplyMergePatch(current, p)
		if err != nil {
			return nil, fmt.Errorf("applying patch %d: %w", i, err)
		}
	}
	return current, nil
}

// StripFields removes specified fields from a JSON document before comparison.
// fieldPaths use dot notation: ".metadata.resourceVersion", ".status"
func StripFields(jsonData []byte, fieldPaths []string, trackStatus bool) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON: %w", err)
	}

	// Always strip default ignore fields
	for _, path := range DefaultIgnoreFields {
		removeField(obj, path)
	}

	// Strip user-specified fields
	for _, path := range fieldPaths {
		removeField(obj, path)
	}

	// Strip status if not tracking
	if !trackStatus {
		delete(obj, "status")
	}

	return json.Marshal(obj)
}

// GetChangedPaths returns top-level paths that differ between two JSON documents.
func GetChangedPaths(oldJSON, newJSON []byte) ([]string, error) {
	var oldObj, newObj map[string]interface{}
	if err := json.Unmarshal(oldJSON, &oldObj); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newJSON, &newObj); err != nil {
		return nil, err
	}

	changed := make(map[string]bool)
	findChanges("", oldObj, newObj, changed, 2)

	paths := make([]string, 0, len(changed))
	for p := range changed {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

// IsEmpty returns true if the merge patch represents no changes.
func IsEmpty(patch []byte) bool {
	if len(patch) == 0 {
		return true
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(patch, &obj); err != nil {
		return false
	}
	return len(obj) == 0
}

// Compress compresses data using gzip.
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress decompresses gzip data.
func Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// --- Internal helpers ---

func removeField(obj map[string]interface{}, path string) {
	path = strings.TrimPrefix(path, ".")
	parts := splitPath(path)
	if len(parts) == 0 {
		return
	}

	current := obj
	for i := 0; i < len(parts)-1; i++ {
		next, ok := current[parts[i]]
		if !ok {
			return
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return
		}
		current = nextMap
	}
	delete(current, parts[len(parts)-1])
}

func splitPath(path string) []string {
	var parts []string
	var current strings.Builder
	escaped := false

	for _, r := range path {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '.' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func findChanges(prefix string, old, new map[string]interface{}, changed map[string]bool, maxDepth int) {
	if maxDepth <= 0 {
		if !jsonEqual(old, new) {
			changed[prefix] = true
		}
		return
	}

	allKeys := make(map[string]bool)
	for k := range old {
		allKeys[k] = true
	}
	for k := range new {
		allKeys[k] = true
	}

	for k := range allKeys {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}

		oldVal, oldExists := old[k]
		newVal, newExists := new[k]

		if !oldExists || !newExists {
			changed[path] = true
			continue
		}

		oldMap, oldIsMap := oldVal.(map[string]interface{})
		newMap, newIsMap := newVal.(map[string]interface{})

		if oldIsMap && newIsMap {
			findChanges(path, oldMap, newMap, changed, maxDepth-1)
		} else if !jsonEqual(oldVal, newVal) {
			changed[path] = true
		}
	}
}

func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return bytes.Equal(aJSON, bJSON)
}
