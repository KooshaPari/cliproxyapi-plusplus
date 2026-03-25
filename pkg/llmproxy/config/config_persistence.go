package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SaveConfigPreserveComments writes the config back to YAML while preserving existing comments
// and key ordering by loading the original file into a yaml.Node tree and updating values in-place.
func SaveConfigPreserveComments(configFile string, cfg *Config) error {
	persistCfg := cfg
	// Load original YAML as a node tree to preserve comments and ordering.
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	var original yaml.Node
	if err = yaml.Unmarshal(data, &original); err != nil {
		return err
	}
	if original.Kind != yaml.DocumentNode || len(original.Content) == 0 {
		return fmt.Errorf("invalid yaml document structure")
	}
	if original.Content[0] == nil || original.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("expected root mapping node")
	}

	// Marshal the current cfg to YAML, then unmarshal to a yaml.Node we can merge from.
	rendered, err := yaml.Marshal(persistCfg)
	if err != nil {
		return err
	}
	var generated yaml.Node
	if err = yaml.Unmarshal(rendered, &generated); err != nil {
		return err
	}
	if generated.Kind != yaml.DocumentNode || len(generated.Content) == 0 || generated.Content[0] == nil {
		return fmt.Errorf("invalid generated yaml structure")
	}
	if generated.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("expected generated root mapping node")
	}

	// Remove deprecated sections before merging back the sanitized config.
	removeLegacyAuthBlock(original.Content[0])
	removeLegacyOpenAICompatAPIKeys(original.Content[0])
	removeLegacyAmpKeys(original.Content[0])
	removeLegacyGenerativeLanguageKeys(original.Content[0])

	pruneMappingToGeneratedKeys(original.Content[0], generated.Content[0], "oauth-excluded-models")
	pruneMappingToGeneratedKeys(original.Content[0], generated.Content[0], "oauth-model-alias")

	// Merge generated into original in-place, preserving comments/order of existing nodes.
	mergeMappingPreserve(original.Content[0], generated.Content[0])
	normalizeCollectionNodeStyles(original.Content[0])

	// Write back.
	f, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err = enc.Encode(&original); err != nil {
		_ = enc.Close()
		return err
	}
	if err = enc.Close(); err != nil {
		return err
	}
	data = NormalizeCommentIndentation(buf.Bytes())
	_, err = f.Write(data)
	return err
}

// SaveConfigPreserveCommentsUpdateNestedScalar updates a nested scalar key path like ["a","b"]
// while preserving comments and positions.
func SaveConfigPreserveCommentsUpdateNestedScalar(configFile string, path []string, value string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	var root yaml.Node
	if err = yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("invalid yaml document structure")
	}
	node := root.Content[0]
	// descend mapping nodes following path
	for i, key := range path {
		if i == len(path)-1 {
			// set final scalar
			v := getOrCreateMapValue(node, key)
			v.Kind = yaml.ScalarNode
			v.Tag = "!!str"
			v.Value = value
		} else {
			next := getOrCreateMapValue(node, key)
			if next.Kind != yaml.MappingNode {
				next.Kind = yaml.MappingNode
				next.Tag = "!!map"
			}
			node = next
		}
	}
	f, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err = enc.Encode(&root); err != nil {
		_ = enc.Close()
		return err
	}
	if err = enc.Close(); err != nil {
		return err
	}
	data = NormalizeCommentIndentation(buf.Bytes())
	_, err = f.Write(data)
	return err
}

// NormalizeCommentIndentation removes indentation from standalone YAML comment lines to keep them left aligned.
func NormalizeCommentIndentation(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	changed := false
	for i, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) == 0 || trimmed[0] != '#' {
			continue
		}
		if len(trimmed) == len(line) {
			continue
		}
		lines[i] = append([]byte(nil), trimmed...)
		changed = true
	}
	if !changed {
		return data
	}
	return bytes.Join(lines, []byte("\n"))
}

// getOrCreateMapValue finds the value node for a given key in a mapping node.
// If not found, it appends a new key/value pair and returns the new value node.
func getOrCreateMapValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode.Kind != yaml.MappingNode {
		mapNode.Kind = yaml.MappingNode
		mapNode.Tag = "!!map"
		mapNode.Content = nil
	}
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			return mapNode.Content[i+1]
		}
	}
	// append new key/value
	mapNode.Content = append(mapNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key})
	val := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}
	mapNode.Content = append(mapNode.Content, val)
	return val
}

// mergeMappingPreserve merges keys from src into dst mapping node while preserving
// key order and comments of existing keys in dst.
func mergeMappingPreserve(dst, src *yaml.Node, path ...[]string) {
	var currentPath []string
	if len(path) > 0 {
		currentPath = path[0]
	}

	if dst == nil || src == nil {
		return
	}
	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		copyNodeShallow(dst, src)
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		sk := src.Content[i]
		sv := src.Content[i+1]
		idx := findMapKeyIndex(dst, sk.Value)
		childPath := appendPath(currentPath, sk.Value)
		if idx >= 0 {
			// Merge into existing value node (always update, even to zero values)
			dv := dst.Content[idx+1]
			mergeNodePreserve(dv, sv, childPath)
		} else {
			// New key: only add if value is non-zero and not a known default
			candidate := deepCopyNode(sv)
			pruneKnownDefaultsInNewNode(childPath, candidate)
			if isKnownDefaultValue(childPath, candidate) {
				continue
			}
			dst.Content = append(dst.Content, deepCopyNode(sk), candidate)
		}
	}
}

// mergeNodePreserve merges src into dst for scalars, mappings and sequences while
// reusing destination nodes to keep comments and anchors.
func mergeNodePreserve(dst, src *yaml.Node, path ...[]string) {
	var currentPath []string
	if len(path) > 0 {
		currentPath = path[0]
	}

	if dst == nil || src == nil {
		return
	}
	switch src.Kind {
	case yaml.MappingNode:
		if dst.Kind != yaml.MappingNode {
			copyNodeShallow(dst, src)
		}
		mergeMappingPreserve(dst, src, currentPath)
	case yaml.SequenceNode:
		// Preserve explicit null style if dst was null and src is empty sequence
		if dst.Kind == yaml.ScalarNode && dst.Tag == "!!null" && len(src.Content) == 0 {
			return
		}
		if dst.Kind != yaml.SequenceNode {
			dst.Kind = yaml.SequenceNode
			dst.Tag = "!!seq"
			dst.Content = nil
		}
		reorderSequenceForMerge(dst, src)
		// Update elements in place
		minContent := len(dst.Content)
		if len(src.Content) < minContent {
			minContent = len(src.Content)
		}
		for i := 0; i < minContent; i++ {
			if dst.Content[i] == nil {
				dst.Content[i] = deepCopyNode(src.Content[i])
				continue
			}
			mergeNodePreserve(dst.Content[i], src.Content[i], currentPath)
			if dst.Content[i] != nil && src.Content[i] != nil &&
				dst.Content[i].Kind == yaml.MappingNode && src.Content[i].Kind == yaml.MappingNode {
				pruneMissingMapKeys(dst.Content[i], src.Content[i])
			}
		}
		// Append any extra items from src
		for i := len(dst.Content); i < len(src.Content); i++ {
			dst.Content = append(dst.Content, deepCopyNode(src.Content[i]))
		}
		// Truncate if dst has extra items not in src
		if len(src.Content) < len(dst.Content) {
			dst.Content = dst.Content[:len(src.Content)]
		}
	case yaml.ScalarNode, yaml.AliasNode:
		// For scalars, update Tag and Value but keep Style from dst
		dst.Kind = src.Kind
		dst.Tag = src.Tag
		dst.Value = src.Value
	default:
		// Fallback: replace shallowly
		copyNodeShallow(dst, src)
	}
}

// findMapKeyIndex returns the index of key node in dst mapping.
// Returns -1 when not found.
func findMapKeyIndex(mapNode *yaml.Node, key string) int {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		if mapNode.Content[i] != nil && mapNode.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// appendPath appends a key to the path.
func appendPath(path []string, key string) []string {
	if len(path) == 0 {
		return []string{key}
	}
	newPath := make([]string, checkedPathLengthPlusOne(len(path)))
	copy(newPath, path)
	newPath[len(path)] = key
	return newPath
}

func checkedPathLengthPlusOne(pathLen int) int {
	maxInt := int(^uint(0) >> 1)
	if pathLen < 0 || pathLen >= maxInt {
		panic(fmt.Sprintf("path length overflow: %d", pathLen))
	}
	return pathLen + 1
}

// isKnownDefaultValue returns true if the given node represents a known default value.
func isKnownDefaultValue(path []string, node *yaml.Node) bool {
	if isZeroValueNode(node) {
		return true
	}

	if len(path) == 0 {
		return false
	}

	fullPath := strings.Join(path, ".")

	// Check string defaults
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		switch fullPath {
		case "pprof.addr":
			return node.Value == DefaultPprofAddr
		case "remote-management.panel-github-repository":
			return node.Value == DefaultPanelGitHubRepository
		case "routing.strategy":
			return node.Value == "round-robin"
		}
	}

	// Check integer defaults
	if node.Kind == yaml.ScalarNode && node.Tag == "!!int" {
		switch fullPath {
		case "error-logs-max-files":
			return node.Value == "10"
		}
	}

	return false
}

// pruneKnownDefaultsInNewNode removes default-valued descendants from a new node.
func pruneKnownDefaultsInNewNode(path []string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		filtered := make([]*yaml.Node, 0, len(node.Content))
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if keyNode == nil || valueNode == nil {
				continue
			}

			childPath := appendPath(path, keyNode.Value)
			if isKnownDefaultValue(childPath, valueNode) {
				continue
			}

			pruneKnownDefaultsInNewNode(childPath, valueNode)
			if (valueNode.Kind == yaml.MappingNode || valueNode.Kind == yaml.SequenceNode) &&
				len(valueNode.Content) == 0 {
				continue
			}

			filtered = append(filtered, keyNode, valueNode)
		}
		node.Content = filtered
	case yaml.SequenceNode:
		for _, child := range node.Content {
			pruneKnownDefaultsInNewNode(path, child)
		}
	}
}

// isZeroValueNode returns true if the YAML node represents a zero/default value.
func isZeroValueNode(node *yaml.Node) bool {
	if node == nil {
		return true
	}
	switch node.Kind {
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!bool":
			return node.Value == "false"
		case "!!int", "!!float":
			return node.Value == "0" || node.Value == "0.0"
		case "!!str":
			return node.Value == ""
		case "!!null":
			return true
		}
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return true
		}
		// Check if all elements are zero values
		for _, child := range node.Content {
			if !isZeroValueNode(child) {
				return false
			}
		}
		return true
	case yaml.MappingNode:
		if len(node.Content) == 0 {
			return true
		}
		// Check if all values are zero values
		for i := 1; i < len(node.Content); i += 2 {
			if !isZeroValueNode(node.Content[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// deepCopyNode creates a deep copy of a yaml.Node graph.
func deepCopyNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	cp := *n
	if len(n.Content) > 0 {
		cp.Content = make([]*yaml.Node, len(n.Content))
		for i := range n.Content {
			cp.Content[i] = deepCopyNode(n.Content[i])
		}
	}
	return &cp
}

// copyNodeShallow copies type/tag/value from src to dst.
func copyNodeShallow(dst, src *yaml.Node) {
	if dst == nil || src == nil {
		return
	}
	dst.Kind = src.Kind
	dst.Tag = src.Tag
	dst.Value = src.Value
	// Replace content with deep copy from src
	if len(src.Content) > 0 {
		dst.Content = make([]*yaml.Node, len(src.Content))
		for i := range src.Content {
			dst.Content[i] = deepCopyNode(src.Content[i])
		}
	} else {
		dst.Content = nil
	}
}

func reorderSequenceForMerge(dst, src *yaml.Node) {
	if dst == nil || src == nil {
		return
	}
	if len(dst.Content) == 0 {
		return
	}
	if len(src.Content) == 0 {
		return
	}
	original := append([]*yaml.Node(nil), dst.Content...)
	used := make([]bool, len(original))
	ordered := make([]*yaml.Node, len(src.Content))
	for i := range src.Content {
		if idx := matchSequenceElement(original, used, src.Content[i]); idx >= 0 {
			ordered[i] = original[idx]
			used[idx] = true
		}
	}
	dst.Content = ordered
}

func matchSequenceElement(original []*yaml.Node, used []bool, target *yaml.Node) int {
	if target == nil {
		return -1
	}
	switch target.Kind {
	case yaml.MappingNode:
		id := sequenceElementIdentity(target)
		if id != "" {
			for i := range original {
				if used[i] || original[i] == nil || original[i].Kind != yaml.MappingNode {
					continue
				}
				if sequenceElementIdentity(original[i]) == id {
					return i
				}
			}
		}
	case yaml.ScalarNode:
		for i := range original {
			if used[i] || original[i] == nil || original[i].Kind != yaml.ScalarNode {
				continue
			}
			if original[i].Value == target.Value {
				return i
			}
		}
	}
	return -1
}

func sequenceElementIdentity(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	// Try to find a unique identifier (e.g., "name" or "id" field)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key == nil || value == nil {
			continue
		}
		if key.Value == "name" && value.Kind == yaml.ScalarNode {
			return value.Value
		}
	}
	return mappingScalarValue(node, "api-key")
}

func mappingScalarValue(node *yaml.Node, key string) string {
	if node == nil || node.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		if k != nil && k.Value == key && v != nil && v.Kind == yaml.ScalarNode {
			return v.Value
		}
	}
	return ""
}

func nodesStructurallyEqual(a, b *yaml.Node) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Kind != b.Kind || a.Tag != b.Tag {
		return false
	}
	if a.Kind == yaml.ScalarNode {
		return a.Value == b.Value
	}
	if len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !nodesStructurallyEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

func removeMapKey(mapNode *yaml.Node, key string) {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return
	}
	newContent := make([]*yaml.Node, 0, len(mapNode.Content))
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		if mapNode.Content[i] != nil && mapNode.Content[i].Value == key {
			continue
		}
		newContent = append(newContent, mapNode.Content[i], mapNode.Content[i+1])
	}
	mapNode.Content = newContent
}

func pruneMappingToGeneratedKeys(dstRoot, srcRoot *yaml.Node, key string) {
	// Find the key in dst and src
	dstIdx := findMapKeyIndex(dstRoot, key)
	srcIdx := findMapKeyIndex(srcRoot, key)

	if dstIdx < 0 {
		return
	}
	if srcIdx < 0 {
		removeMapKey(dstRoot, key)
		return
	}
}

func pruneMissingMapKeys(dstMap, srcMap *yaml.Node) {
	if dstMap == nil || srcMap == nil {
		return
	}
	if dstMap.Kind != yaml.MappingNode || srcMap.Kind != yaml.MappingNode {
		return
	}

	// Build set of keys in src
	srcKeys := make(map[string]struct{})
	for i := 0; i+1 < len(srcMap.Content); i += 2 {
		if srcMap.Content[i] != nil {
			srcKeys[srcMap.Content[i].Value] = struct{}{}
		}
	}

	// Remove keys from dst that are not in src
	newContent := make([]*yaml.Node, 0, len(dstMap.Content))
	for i := 0; i+1 < len(dstMap.Content); i += 2 {
		if dstMap.Content[i] != nil && dstMap.Content[i].Value != "" {
			if _, exists := srcKeys[dstMap.Content[i].Value]; exists {
				newContent = append(newContent, dstMap.Content[i], dstMap.Content[i+1])
			}
		}
	}
	dstMap.Content = newContent
}

func normalizeCollectionNodeStyles(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		node.Style = 0
		for _, child := range node.Content {
			normalizeCollectionNodeStyles(child)
		}
	case yaml.SequenceNode:
		node.Style = 0
		for _, child := range node.Content {
			normalizeCollectionNodeStyles(child)
		}
	}
}

// Legacy removal functions
func removeLegacyOpenAICompatAPIKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "openai-compatibility-api-keys")
}

func removeLegacyAmpKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "amp-upstream-url")
	removeMapKey(root, "amp-upstream-api-key")
	removeMapKey(root, "amp-restrict-management-to-localhost")
	removeMapKey(root, "amp-model-mappings")
}

func removeLegacyGenerativeLanguageKeys(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "generative-language-api-key")
}

func removeLegacyAuthBlock(root *yaml.Node) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	removeMapKey(root, "auth")
}
