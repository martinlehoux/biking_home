package osmpass

import (
	"testing"

	"github.com/paulmach/osm"
	"github.com/stretchr/testify/assert"
)

func TestIsMountainPass(t *testing.T) {
	cases := []struct {
		name string
		tags map[string]string
		want bool
	}{
		{"tagged", map[string]string{"mountain_pass": "yes", "name": "Col de la Gineste"}, true},
		{"legacy natural", map[string]string{"natural": "mountain_pass"}, true},
		{"not a pass", map[string]string{"natural": "saddle", "name": "Col de la Gineste"}, false},
		{"no tags", map[string]string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := &osm.Node{Tags: osm.Tags{}}
			for key, value := range c.tags {
				node.Tags = append(node.Tags, osm.Tag{Key: key, Value: value})
			}
			assert.Equal(t, c.want, isMountainPass(node))
		})
	}
}

func TestParseElevation(t *testing.T) {
	cases := []struct {
		input string
		want  int
		ok    bool
	}{
		{"1320", 1320, true},
		{"1320 m", 1320, true},
		{" 447.5", 447, true},
		{"", 0, false},
		{"unknown", 0, false},
	}
	for _, c := range cases {
		value, ok := parseElevation(c.input)
		assert.Equal(t, c.want, value)
		assert.Equal(t, c.ok, ok)
	}
}
