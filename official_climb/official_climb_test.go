package official_climb_test

import (
	"testing"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/internal/dbtest"
	"github.com/martinlehoux/biking_home/official_climb"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateListAndGetOfficialClimb(t *testing.T) {
	db := dbtest.New(t)
	created, err := official_climb.Create(db, official_climb.OfficialClimb{
		Name:       "Col de Test",
		StartCoord: geodist.Coord{Lat: 43.1, Lon: 5.1},
		EndCoord:   geodist.Coord{Lat: 43.2, Lon: 5.2},
	})
	require.NoError(t, err)
	assert.Positive(t, created.ID)
	assert.Equal(t, "Col de Test", created.Name)
	assert.Equal(t, geodist.Coord{Lat: 43.1, Lon: 5.1}, created.StartCoord)

	listed, err := official_climb.List(db)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	got, found, err := official_climb.GetByID(db, created.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, created.EndCoord, got.EndCoord)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

func TestCreateRejectsInvalidOfficialClimb(t *testing.T) {
	db := dbtest.New(t)
	_, err := official_climb.Create(db, official_climb.OfficialClimb{
		Name:       " ",
		StartCoord: geodist.Coord{Lat: 43.1, Lon: 5.1},
		EndCoord:   geodist.Coord{Lat: 43.2, Lon: 5.2},
	})
	assert.EqualError(t, err, "official climb name is required")
}

func TestMatchClimbUsesOrderedCoordinates(t *testing.T) {
	climb := testClimb(
		geodist.Coord{Lat: 43.1002, Lon: 5.1002},
		geodist.Coord{Lat: 43.2002, Lon: 5.2002},
	)
	match, found := official_climb.MatchClimb(climb, []official_climb.OfficialClimb{
		{Name: "Same direction", StartCoord: geodist.Coord{Lat: 43.1, Lon: 5.1}, EndCoord: geodist.Coord{Lat: 43.2, Lon: 5.2}},
		{Name: "Reverse direction", StartCoord: geodist.Coord{Lat: 43.2, Lon: 5.2}, EndCoord: geodist.Coord{Lat: 43.1, Lon: 5.1}},
	}, official_climb.DefaultMatchPolicy())

	require.True(t, found)
	assert.Equal(t, "Same direction", match.Name)
}

func TestMatchClimbRejectsEndpointOutsideRadius(t *testing.T) {
	climb := testClimb(
		geodist.Coord{Lat: 43.1, Lon: 5.1},
		geodist.Coord{Lat: 43.3, Lon: 5.3},
	)
	_, found := official_climb.MatchClimb(climb, []official_climb.OfficialClimb{{
		Name:       "Partial match",
		StartCoord: geodist.Coord{Lat: 43.1, Lon: 5.1},
		EndCoord:   geodist.Coord{Lat: 43.2, Lon: 5.2},
	}}, official_climb.DefaultMatchPolicy())
	assert.False(t, found)
}

func TestMatchClimbFindsOfficialEndpointsInsideDetectedClimb(t *testing.T) {
	parsed := ride.FromColumns(
		[]float64{0, 1000, 2000, 3000},
		[]float64{100, 300, 500, 100},
		[]geodist.Coord{
			{Lat: 43.1, Lon: 5.1},
			{Lat: 43.101, Lon: 5.101},
			{Lat: 43.102, Lon: 5.102},
			{Lat: 43.103, Lon: 5.103},
		},
		[]time.Time{time.Unix(0, 0), time.Unix(60, 0), time.Unix(120, 0), time.Unix(180, 0)},
	)
	climb := parsed.ClimbFromIndexes(0, 3)
	matched, found := official_climb.MatchClimb(climb, []official_climb.OfficialClimb{{
		Name:       "Corrected boundaries",
		StartCoord: parsed.Coord(1),
		EndCoord:   parsed.Coord(2),
	}}, official_climb.DefaultMatchPolicy())

	require.True(t, found)
	assert.Equal(t, "Corrected boundaries", matched.Name)
}

func TestMatchClimbMatchesSameOfficialClimbAcrossRides(t *testing.T) {
	official := official_climb.OfficialClimb{
		Name:       "Shared official climb",
		StartCoord: geodist.Coord{Lat: 43.100, Lon: 5.100},
		EndCoord:   geodist.Coord{Lat: 43.200, Lon: 5.200},
	}
	firstRide := ride.FromColumns(
		[]float64{0, 1000, 2000, 3000},
		[]float64{100, 300, 500, 100},
		[]geodist.Coord{
			{Lat: 43.090, Lon: 5.090},
			official.StartCoord,
			official.EndCoord,
			{Lat: 43.210, Lon: 5.210},
		},
		[]time.Time{time.Unix(0, 0), time.Unix(60, 0), time.Unix(120, 0), time.Unix(180, 0)},
	)
	secondRide := ride.FromColumns(
		[]float64{0, 750, 1500, 2250, 3000},
		[]float64{100, 180, 300, 500, 100},
		[]geodist.Coord{
			{Lat: 43.080, Lon: 5.080},
			{Lat: 43.095, Lon: 5.095},
			official.StartCoord,
			official.EndCoord,
			{Lat: 43.205, Lon: 5.205},
		},
		[]time.Time{time.Unix(0, 0), time.Unix(45, 0), time.Unix(90, 0), time.Unix(135, 0), time.Unix(180, 0)},
	)

	firstMatch, firstFound := official_climb.MatchClimb(firstRide.ClimbFromIndexes(0, 3), []official_climb.OfficialClimb{official}, official_climb.DefaultMatchPolicy())
	secondMatch, secondFound := official_climb.MatchClimb(secondRide.ClimbFromIndexes(1, 4), []official_climb.OfficialClimb{official}, official_climb.DefaultMatchPolicy())

	require.True(t, firstFound)
	require.True(t, secondFound)
	assert.Equal(t, official.Name, firstMatch.Name)
	assert.Equal(t, official.Name, secondMatch.Name)
}

func TestMatchClimbChoosesNearestCandidate(t *testing.T) {
	climb := testClimb(
		geodist.Coord{Lat: 43.1, Lon: 5.1},
		geodist.Coord{Lat: 43.2, Lon: 5.2},
	)
	match, found := official_climb.MatchClimb(climb, []official_climb.OfficialClimb{
		{ID: 2, Name: "Farther", StartCoord: geodist.Coord{Lat: 43.1005, Lon: 5.1005}, EndCoord: geodist.Coord{Lat: 43.2005, Lon: 5.2005}},
		{ID: 1, Name: "Nearest", StartCoord: geodist.Coord{Lat: 43.1001, Lon: 5.1001}, EndCoord: geodist.Coord{Lat: 43.2001, Lon: 5.2001}},
	}, official_climb.DefaultMatchPolicy())

	require.True(t, found)
	assert.Equal(t, "Nearest", match.Name)
}

func TestMatchPolicyRejectsInvalidRadius(t *testing.T) {
	assert.EqualError(t, (official_climb.MatchPolicy{}).Validate(), "official climb endpoint radius must be greater than zero")
}

func testClimb(start, end geodist.Coord) ride.Climb {
	parsed := ride.FromColumns(
		[]float64{0, 1000, 2000},
		[]float64{100, 150, 200},
		[]geodist.Coord{{}, start, end},
		[]time.Time{time.Unix(0, 0), time.Unix(60, 0), time.Unix(120, 0)},
	)
	return parsed.ClimbFromDist(1000, 2000)
}
