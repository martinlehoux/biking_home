package rides

import (
	"database/sql"
	"testing"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`
		create table rides (
			id integer primary key,
			external_id text unique not null,
			gpx_path text not null,
			name text not null,
			type text not null,
			start_date text not null,
			distance_m real not null,
			moving_time_s integer not null,
			elapsed_time_s integer not null,
			total_elevation_gain_m real not null,
			average_speed_mps real not null,
			created_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
			updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)
	`)
	require.NoError(t, err)
	return db
}

func sampleRide() Ride {
	return Ride{
		ExternalID:          "strava:1234",
		GPXPath:             "rides/activity_1234.gpx",
		Name:                "Morning Ride",
		Type:                "Ride",
		StartDate:           time.Date(2026, 8, 1, 7, 30, 0, 0, time.UTC),
		DistanceM:           42_195,
		MovingTimeS:         7_200,
		ElapsedTimeS:        7_800,
		TotalElevationGainM: 850,
		AverageSpeedMps:     5.86,
	}
}

func TestUpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	err := Save(db, sampleRide())
	require.NoError(t, err)

	got, ok, err := GetByExternalID(db, "strava:1234")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Morning Ride", got.Name)
	assert.Equal(t, sampleRide().StartDate, got.StartDate)
	assert.Equal(t, 42_195.0, got.DistanceM)
	assert.Equal(t, "rides/activity_1234.gpx", got.GPXPath)

	_, ok, err = GetByExternalID(db, "strava:9999")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUpsertUpdatesExisting(t *testing.T) {
	db := newTestDB(t)
	ride := sampleRide()
	require.NoError(t, Save(db, ride))
	ride.Name = "Renamed Ride"
	ride.DistanceM = 50_000
	require.NoError(t, Save(db, ride))

	got, ok, err := GetByExternalID(db, "strava:1234")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Renamed Ride", got.Name)
	assert.Equal(t, 50_000.0, got.DistanceM)
}

func TestList(t *testing.T) {
	db := newTestDB(t)
	first := sampleRide()
	first.StartDate = time.Date(2026, 7, 1, 7, 0, 0, 0, time.UTC)
	second := sampleRide()
	second.ExternalID = "strava:5678"
	second.Name = "Evening Ride"
	require.NoError(t, Save(db, first))
	require.NoError(t, Save(db, second))

	rides, err := List(db)
	require.NoError(t, err)
	require.Len(t, rides, 2)
	assert.Equal(t, "strava:5678", rides[0].ExternalID)
	assert.Equal(t, "strava:1234", rides[1].ExternalID)
	kcore.Assert(len(rides) == 2, "two rides")
}
