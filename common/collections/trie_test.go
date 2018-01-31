package collections_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/collections"
)

func newTrie() *collections.Trie {
	trie := collections.NewTrie()
	trie.Add("1.5.8")
	trie.Add("1.6.13")
	trie.Add("1.7.12")
	trie.Add("1.8.6")
	trie.Add("1.9.1")
	trie.Add("1.9.2")
	return trie
}

func TestLongestMatch(t *testing.T) {
	trie := newTrie()

	longestMatch, isPerfectMatch := trie.LongestMatch("")
	assert.Equal(t, "", longestMatch)
	assert.True(t, isPerfectMatch)

	longestMatch, isPerfectMatch = trie.LongestMatch("1.8.5")
	assert.Equal(t, "1.8.", longestMatch)
	assert.False(t, isPerfectMatch)

	longestMatch, isPerfectMatch = trie.LongestMatch("1.8.6")
	assert.Equal(t, "1.8.6", longestMatch)
	assert.True(t, isPerfectMatch)

	longestMatch, isPerfectMatch = trie.LongestMatch("1.8.6-alpha")
	assert.Equal(t, "1.8.6", longestMatch)
	assert.False(t, isPerfectMatch)

	longestMatch, isPerfectMatch = trie.LongestMatch("1.9.3")
	assert.Equal(t, "1.9.", longestMatch)
	assert.False(t, isPerfectMatch)

	longestMatch, isPerfectMatch = trie.LongestMatch("foobar")
	assert.Equal(t, "", longestMatch)
	assert.False(t, isPerfectMatch)
}

func TestBestMatch(t *testing.T) {
	trie := newTrie()

	bestMatch, isPerfectMatchOrAutoComplete := trie.BestMatch("")
	assert.Equal(t, "", bestMatch)
	assert.True(t, isPerfectMatchOrAutoComplete)

	bestMatch, isPerfectMatchOrAutoComplete = trie.BestMatch("1.8.5")
	assert.Equal(t, "1.8.6", bestMatch)
	assert.True(t, isPerfectMatchOrAutoComplete)

	bestMatch, isPerfectMatchOrAutoComplete = trie.BestMatch("1.8.6")
	assert.Equal(t, "1.8.6", bestMatch)
	assert.True(t, isPerfectMatchOrAutoComplete)

	bestMatch, isPerfectMatchOrAutoComplete = trie.BestMatch("1.8.6-alpha")
	assert.Equal(t, "1.8.6", bestMatch)
	assert.True(t, isPerfectMatchOrAutoComplete)

	bestMatch, isPerfectMatchOrAutoComplete = trie.BestMatch("1.9.3")
	assert.Equal(t, "1.9.", bestMatch)
	assert.False(t, isPerfectMatchOrAutoComplete)

	bestMatch, isPerfectMatchOrAutoComplete = trie.BestMatch("foobar")
	assert.Equal(t, "", bestMatch)
	assert.False(t, isPerfectMatchOrAutoComplete)
}
