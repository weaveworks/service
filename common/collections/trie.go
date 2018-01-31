package collections

// NewTrie creates... a new trie.
func NewTrie() *Trie {
	return &Trie{
		root: &Node{
			children: make(map[rune]*Node),
		},
	}
}

// Trie represents a tree of runes (i.e. characters) and allows efficient prefix searches on these.
type Trie struct {
	root *Node
}

// Node is an element within a trie.
type Node struct {
	value    rune
	children map[rune]*Node
}

// Add adds the provided string, rune by rune, to this trie.
func (t *Trie) Add(str string) {
	node := t.root
	runes := []rune(str)
	for i := 0; i < len(runes); i++ {
		next, ok := node.children[runes[i]]
		if !ok {
			next = &Node{
				value:    runes[i],
				children: make(map[rune]*Node),
			}
			node.children[runes[i]] = next
		}
		node = next
	}
}

// LongestMatch traverses the trie using the provided string, rune by rune, to find the longest matching string.
// It returns the longest match and a boolean indicating whether this is a perfect match or not.
func (t Trie) LongestMatch(str string) (string, bool) {
	var longestMatch []rune
	node := t.root
	runes := []rune(str)
	for i := 0; i < len(runes); i++ {
		if next, ok := node.children[runes[i]]; ok {
			longestMatch = append(longestMatch, runes[i])
			node = next
		} else {
			return string(longestMatch), false
		}
	}
	return string(longestMatch), true
}

// BestMatch traverses trie using the provided string, rune by rune, to find the best matching string.
// It auto-completes it if there is an unique leaf to the matching branch.
// It returns the best match and a boolean indicating:
// - true if it is a perfect match
// - true if it was able to auto-complete
// - false otherwise.
func (t Trie) BestMatch(str string) (string, bool) {
	var bestMatch []rune
	isPerfectMatch := true
	node := t.root
	runes := []rune(str)
	for i := 0; i < len(runes); i++ {
		if next, ok := node.children[runes[i]]; ok {
			bestMatch = append(bestMatch, runes[i])
			node = next
		} else {
			isPerfectMatch = false
			break
		}
	}

	if isPerfectMatch {
		return string(bestMatch), true
	}

	// A perfect match could not be found. We continue to traverse the trie in
	// order to check whether there is an unique leaf from where we are in the
	// current branch, in which case we can auto-complete the match.

	var remainder []rune
	for len(node.children) == 1 {
		// Even though we have a loop, we're really just moving on to the only available child node:
		for key := range node.children {
			node = node.children[key]
		}
		remainder = append(remainder, node.value)
	}

	if len(node.children) == 0 {
		// We have reached the leaf node in the branch currently matching, we can
		// therefore auto-complete the current best match with the remainder:
		bestMatch = append(bestMatch, remainder...)
		return string(bestMatch), true
	}

	// If there is more than one leaf in the branch currently matching, we cannot auto-complete, but anyway
	// all the matching runes are in bestMatch, so we don't need to do anything special to handle this case.
	return string(bestMatch), false
}
