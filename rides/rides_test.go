package rides

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/martinlehoux/biking_home/ride"
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
			cotacol_score real,
			cotacol_algo_version text,
			created_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
			updated_at text not null default (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		)
	`)
	require.NoError(t, err)
	return db
}

func sampleRide(t *testing.T) Ride {
	t.Helper()
	gpxPath := filepath.Join(t.TempDir(), "ride.gpx")
	require.NoError(t, os.WriteFile(gpxPath, []byte(testGPX), 0o600))
	return Ride{
		ExternalID:          "strava:1234",
		GPXPath:             gpxPath,
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

const testGPX = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg><trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt><trkpt lat="43.001" lon="5.001"><ele>200</ele></trkpt></trkseg></trk></gpx>`

func TestUpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	sample := sampleRide(t)
	err := Save(db, sample)
	require.NoError(t, err)

	got, ok, err := GetByExternalID(db, "strava:1234")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "Morning Ride", got.Name)
	assert.Equal(t, sample.StartDate, got.StartDate)
	assert.Equal(t, 42_195.0, got.DistanceM)
	assert.Equal(t, sample.GPXPath, got.GPXPath)
	score, found := got.CotacolScore()
	require.True(t, found)
	assert.Greater(t, score, 0.0)
	assert.Equal(t, ride.CotacolAlgorithmVersion, got.CotacolAlgorithmVersion())

	_, ok, err = GetByExternalID(db, "strava:9999")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSaveComputesCotacol(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, Save(db, sampleRide(t)))

	got, ok, err := GetByExternalID(db, "strava:1234")
	require.NoError(t, err)
	require.True(t, ok)
	score, found := got.CotacolScore()
	require.True(t, found)
	assert.Greater(t, score, 0.0)
	assert.Equal(t, ride.CotacolAlgorithmVersion, got.CotacolAlgorithmVersion())
}

func TestBackfill(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, Save(db, sampleRide(t)))

	count, err := Backfill(db)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	got, ok, err := GetByExternalID(db, "strava:1234")
	require.NoError(t, err)
	require.True(t, ok)
	_, found := got.CotacolScore()
	assert.True(t, found)
	assert.Equal(t, ride.CotacolAlgorithmVersion, got.CotacolAlgorithmVersion())

	count, err = Backfill(db)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUpsertUpdatesExisting(t *testing.T) {
	db := newTestDB(t)
	ride := sampleRide(t)
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
	first := sampleRide(t)
	first.StartDate = time.Date(2026, 7, 1, 7, 0, 0, 0, time.UTC)
	second := sampleRide(t)
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
