package strava

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tkrajina/gpxgo/gpx"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient("12345", "secret", Token{
		AccessToken:  "fresh-access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	apiEndpoint = server.URL
	return client
}

func TestListActivities(t *testing.T) {
	calls := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/athlete/activities", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "200", q.Get("per_page"))
		assert.Equal(t, fmt.Sprint(calls), q.Get("page"))
		assert.Equal(t, "1600000000", q.Get("after"))
		w.Header().Set("Content-Type", "application/json")
		body := `[{"id":1,"name":"Ride A","type":"Ride","sport_type":"Ride","start_date":"2026-01-01T10:00:00Z"},{"id":2,"name":"Run","type":"Run","sport_type":"Run","start_date":"2026-01-01T11:00:00Z"}]`
		if calls == 2 {
			body = `[]`
		}
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	})

	activities, err := client.ListActivities(time.Unix(1600000000, 0), "Ride")
	require.NoError(t, err)
	require.Len(t, activities, 1)
	assert.Equal(t, int64(1), activities[0].ID)
	assert.Equal(t, "Ride A", activities[0].Name)
	assert.Equal(t, time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), activities[0].StartDate)
}

func TestActivityStreams(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/activities/42/streams", r.URL.Path)
		assert.Equal(t, "latlng,altitude,time", r.URL.Query().Get("keys"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"latlng":{"type":"latlng","data":[[43.0,5.0],[43.1,5.1],[43.2,5.2]]},
			"altitude":{"type":"altitude","data":[100,120,140]},
			"time":{"type":"time","data":[0,10,25]}
		}`))
		require.NoError(t, err)
	})

	latlng, altitude, seconds, err := client.ActivityStreams(42)
	require.NoError(t, err)
	require.Len(t, latlng, 3)
	assert.Equal(t, [2]float64{43.0, 5.0}, latlng[0])
	assert.Equal(t, []float64{100, 120, 140}, altitude)
	assert.Equal(t, []float64{0, 10, 25}, seconds)
}

func TestWriteActivityGPX(t *testing.T) {
	activity := Activity{ID: 42, Name: "Ride A", StartDate: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}
	latlng := [][2]float64{{43.0, 5.0}, {43.1, 5.1}}
	altitude := []float64{100, 120}
	seconds := []float64{0, 10}

	file := filepath.Join(t.TempDir(), "activity_42.gpx")
	err := WriteActivityGPX(file, activity, latlng, altitude, seconds)
	require.NoError(t, err)

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	parsed, err := gpx.ParseBytes(data)
	require.NoError(t, err)
	require.Len(t, parsed.Tracks, 1)
	require.Len(t, parsed.Tracks[0].Segments, 1)
	points := parsed.Tracks[0].Segments[0].Points
	require.Len(t, points, 2)
	assert.Equal(t, 43.0, points[0].Latitude)
	assert.Equal(t, 5.1, points[1].Longitude)
	assert.Equal(t, 100.0, points[0].Elevation.Value())
	assert.Equal(t, activity.StartDate.Add(10*time.Second), points[1].Timestamp)
	assert.Equal(t, "Ride A", parsed.Name)
}
