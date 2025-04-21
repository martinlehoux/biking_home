package ride_test

import (
	"fmt"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
)

func TestRideClimbLaCride(t *testing.T) {
	ride19Jan, err := ride.ParseFile(parser, "../examples/activity_18043356988.gpx")
	assert.NoError(t, err)
	ride30Mar, err := ride.ParseFile(parser, "../examples/activity_18679866717.gpx")
	assert.NoError(t, err)
	ride20Apr, err := ride.ParseFile(parser, "../examples/activity_18880605641.gpx")
	assert.NoError(t, err)
	index := ride.NewFlatRideClimbsIndex(30)
	index.Insert(ride19Jan)
	index.Insert(ride30Mar)
	logs := []string{}
	for _, climb := range ride20Apr.AllClimbs() {
		logs = append(logs, fmt.Sprintf("Climb %s (%s, %.1fkm/h)", climb, climb.Duration(), climb.Speed()*3.6))
		if len(index.Similar(ride20Apr, climb)) > 0 {
			for _, similar := range index.Similar(ride20Apr, climb) {
				logs = append(logs, fmt.Sprintf("  %s (%s, %.1fkm/h)", similar, similar.Duration(), similar.Speed()*3.6))
			}
		}
	}
	cupaloy.SnapshotT(t, logs)
}
