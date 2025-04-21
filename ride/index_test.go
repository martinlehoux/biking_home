package ride_test

import (
	"fmt"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
	"github.com/tkrajina/gpxgo/gpx"
)

func TestRideClimbLaCride(t *testing.T) {
	gpxContent, err := gpx.ParseFile("../examples/activity_18043356988.gpx")
	assert.NoError(t, err)
	ride19Jan := ride.FromGPX(gpxContent)
	gpxContent, err = gpx.ParseFile("../examples/activity_18679866717.gpx")
	assert.NoError(t, err)
	ride30Mar := ride.FromGPX(gpxContent)
	gpxContent, err = gpx.ParseFile("../examples/activity_18880605641.gpx")
	assert.NoError(t, err)
	ride20Apr := ride.FromGPX(gpxContent)
	index := ride.NewFlatRideClimbsIndex(30)
	index.Insert(ride19Jan)
	index.Insert(ride30Mar)
	logs := []string{}
	for _, climb := range ride20Apr.AllClimbs() {
		if len(index.Similar(ride20Apr, climb)) > 0 {
			logs = append(logs, fmt.Sprintf("Found similar climbs to %s", climb))
			for _, similar := range index.Similar(ride20Apr, climb) {
				logs = append(logs, fmt.Sprintf("  %s", similar))
			}
		}
	}
	cupaloy.SnapshotT(t, logs)
}
