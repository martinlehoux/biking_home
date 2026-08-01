package ride_test

import (
	"testing"

	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
)

func TestDifficultyScoreConstantClimb(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("2km at 7%").Build()
	assert.InDelta(t, 98.0, r.DifficultyScore(), 1e-9)
}

func TestDifficultyScoreIgnoresDescent(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("2km at 7%").WithSection("2km at -7%").Build()
	assert.InDelta(t, 98.0, r.DifficultyScore(), 1e-9)
}

func TestDifficultyScoreFlatIsZero(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("5km at 0%").Build()
	assert.Zero(t, r.DifficultyScore())
}

func TestDifficultyScoreSteepShortCountsMore(t *testing.T) {
	steep := RideBuilder{precision: 100}.WithSection("0.5km at 14%").Build()
	long := RideBuilder{precision: 100}.WithSection("1km at 7%").Build()
	assert.InDelta(t, 98.0, steep.DifficultyScore(), 1e-9)
	assert.InDelta(t, 49.0, long.DifficultyScore(), 1e-9)
}

func TestDifficultyScorePrecisionInvariant(t *testing.T) {
	coarse := RideBuilder{precision: 100}.WithSection("3km at 5%").Build()
	fine := RideBuilder{precision: 50}.WithSection("3km at 5%").Build()
	assert.InDelta(t, coarse.DifficultyScore(), fine.DifficultyScore(), 1e-9)
}

func TestDifficultyScoreExampleRide(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2022-07-21.Pogacar.gpx")
	assert.NoError(t, err)
	assert.InDelta(t, 3049.0, r.DifficultyScore(), 1)
}
