package mountain_pass_test

import (
	"testing"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/internal/dbtest"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCrossingsFindsPass(t *testing.T) {
	r := rideFromPoint(t, 43.62, 5.43, 447)
	pass := mountain_pass.MountainPass{
		Name: "Pas de Magnan", Elevation: 460,
		Coord: &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}
	crossings := mountain_pass.DetectCrossings(r, []mountain_pass.MountainPass{pass}, 100, 25)
	assert.Len(t, crossings, 1)
	assert.Equal(t, "Pas de Magnan", crossings[0].Pass.Name)
	assert.Less(t, crossings[0].DistanceToM, 10.0)
	assert.Less(t, crossings[0].ElevationDiff, 25.0)
}

func TestLoadMountainPassesAroundRideUsesExpandedBounds(t *testing.T) {
	db := dbtest.New(t)
	_, err := db.Exec(`
		INSERT INTO mountain_passes (external_id, name, country_code, department_code, elevation, latitude, longitude)
		VALUES (?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?, ?)
	`,
		"550e8400-e29b-41d4-a716-446655440000", "On route", "FR", "13", 400, 43.05, 5.05,
		"6ba7b810-9dad-41d1-80b4-00c04fd430c8", "Within margin", "FR", "13", 500, 43.18, 5.20,
		"6ba7b811-9dad-41d1-80b4-00c04fd430c8", "Outside margin", "FR", "13", 600, 43.21, 5.20,
		"6ba7b812-9dad-41d1-80b4-00c04fd430c8", "Missing coordinates", "FR", "13", 700, nil, nil,
		"6ba7b813-9dad-41d1-80b4-00c04fd430c8", "On route edge", "FR", "13", 300, 43.10, 5.10,
	)
	require.NoError(t, err)

	route := ride.FromColumns(
		[]float64{0, 1000},
		[]float64{100, 200},
		[]geodist.Coord{{Lat: 43.0, Lon: 5.0}, {Lat: 43.1, Lon: 5.1}},
		make([]time.Time, 2),
	)
	passes, err := mountain_pass.LoadMountainPassesAroundRide(db, route, 10_000)
	require.NoError(t, err)
	require.Len(t, passes, 3)
	assert.Equal(t, "On route edge", passes[0].Name)
	assert.Contains(t, []string{passes[1].Name, passes[2].Name}, "On route")
	assert.Contains(t, []string{passes[1].Name, passes[2].Name}, "Within margin")
}

func TestDetectCrossingsIgnoresDistantPass(t *testing.T) {
	r := rideFromPoint(t, 43.62, 5.43, 447)
	far := mountain_pass.MountainPass{
		Name: "Col de la Faucille", Elevation: 1320,
		Coord: &geodist.Coord{Lat: 46.37, Lon: 6.02},
	}
	assert.Empty(t, mountain_pass.DetectCrossings(r, []mountain_pass.MountainPass{far}, 100, 25))
}

func TestDetectCrossingsIgnoresPassWithoutCoordinates(t *testing.T) {
	r := rideFromPoint(t, 43.62, 5.43, 447)
	noCoord := mountain_pass.MountainPass{Name: "Col de la Gatasse", Elevation: 120}
	assert.Empty(t, mountain_pass.DetectCrossings(r, []mountain_pass.MountainPass{noCoord}, 100, 25))
}

func TestDetectCrossingsRequiresElevationAgreement(t *testing.T) {
	r := rideFromPoint(t, 43.62, 5.43, 447)
	pass := mountain_pass.MountainPass{
		Name: "Pas de Magnan", Elevation: 900,
		Coord: &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}
	assert.Empty(t, mountain_pass.DetectCrossings(r, []mountain_pass.MountainPass{pass}, 100, 25))
}

func rideFromPoint(t *testing.T, latitude, longitude, elevation float64) ride.Ride {
	t.Helper()
	return ride.FromColumns(
		[]float64{0, 1000},
		[]float64{100, elevation},
		[]geodist.Coord{{Lat: latitude - 0.01, Lon: longitude - 0.01}, {Lat: latitude, Lon: longitude}},
		make([]time.Time, 2),
	)
}

func TestMatchClimbFindsPassAtTop(t *testing.T) {
	climb := climbFromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.63, Lon: 5.44}},
	)
	pass := mountain_pass.MountainPass{
		Name: "Pas de Magnan", Elevation: 440,
		Coord: &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}
	matched, found := mountain_pass.MatchClimb(climb, []mountain_pass.MountainPass{pass}, 100, 25)
	assert.True(t, found)
	assert.Equal(t, "Pas de Magnan", matched.Name)
}

func TestMatchClimbRequiresElevationAgreement(t *testing.T) {
	climb := climbFromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.63, Lon: 5.44}},
	)
	pass := mountain_pass.MountainPass{
		Name: "Pas de Magnan", Elevation: 900,
		Coord: &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}
	_, found := mountain_pass.MatchClimb(climb, []mountain_pass.MountainPass{pass}, 100, 25)
	assert.False(t, found)
}

func TestMatchClimbNearestOfTwo(t *testing.T) {
	climb := climbFromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.63, Lon: 5.44}},
	)
	near := mountain_pass.MountainPass{
		Name: "Pas de Magnan", Elevation: 440,
		Coord: &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}
	far := mountain_pass.MountainPass{
		Name: "Col lointain", Elevation: 440,
		Coord: &geodist.Coord{Lat: 43.6201, Lon: 5.4301},
	}
	matched, found := mountain_pass.MatchClimb(climb, []mountain_pass.MountainPass{near, far}, 100, 25)
	assert.True(t, found)
	assert.Equal(t, "Pas de Magnan", matched.Name)
}

func climbFromColumns(distances, elevations []float64, coords []geodist.Coord) ride.Climb {
	ride := ride.FromColumns(distances, elevations, coords, make([]time.Time, len(distances)))
	return ride.ClimbFromDist(0, 2000)
}
