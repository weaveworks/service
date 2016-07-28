package externalIDs

import (
	"fmt"
	"math/rand"
)

var (
	adjs = [...]string{
		"able", "aged", "ancient", "autumn", "big", "billowing", "bitter", "black",
		"blue", "bold", "brave", "brief", "bright", "brilliant", "brisk", "broken",
		"calm", "careful", "charming", "chilly", "cold", "cool", "courageous", "crimson",
		"damp", "dapper", "dark", "dawn", "dazzling", "delicate", "divine", "dry",
		"eager", "empty", "falling", "fancy", "fast", "fearless", "feisty", "floating",
		"floral", "fragrant", "frosty", "frozen", "gentle", "giant", "glass", "green",
		"happy", "hidden", "holy", "icy", "jolly", "joyful", "late", "light",
		"lingering", "little", "lively", "long", "loud", "majestic", "merry", "misty",
		"modern", "morning", "muddy", "nameless", "natural", "new", "old", "orange",
		"pale", "patient", "peaceful", "polished", "proud", "purple", "quiet", "red",
		"restless", "rough", "shy", "silent", "small", "snowy", "solitary", "sparkling",
		"spring", "still", "summer", "thawing", "timely", "twilight", "wandering", "weathered",
		"white", "wild", "winter", "wispy", "withered", "young",
	}
	nouns = [...]string{
		"bird", "blossom", "breeze", "brook", "butterfly", "cloud", "darkness", "dawn",
		"dew", "dream", "dust", "feather", "field", "fire", "firefly", "flower",
		"fog", "forest", "frog", "frost", "glade", "glitter", "grape", "grass",
		"haze", "hill", "lake", "leaf", "meadow", "meteor", "moon", "morning",
		"mountain", "nebula", "night", "paper", "pine", "planet", "pond", "pulsar",
		"rain", "resonance", "river", "rock", "sea", "shadow", "shape", "silence",
		"sky", "smoke", "snow", "snowflake", "sound", "star", "stone", "sun",
		"sunset", "surf", "thunder", "tree", "vine", "violet", "voice", "water",
		"waterfall", "wave", "wildflower", "wind",
	}
)

// Generate a new id of the form autumn-waterfall-99
func Generate() string {
	return fmt.Sprintf(
		"%s-%s-%02d",
		adjs[rand.Intn(len(adjs))],
		nouns[rand.Intn(len(nouns))],
		rand.Int31n(100),
	)
}
