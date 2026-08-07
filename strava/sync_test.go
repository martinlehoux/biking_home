package strava

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/martinlehoux/biking_home/ride"
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

func TestList(t *testing.T) {
	calls := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/athlete/activities", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, "200", q.Get("per_page"))
		assert.Equal(t, fmt.Sprint(calls), q.Get("page"))
		assert.Equal(t, "1600000000", q.Get("after"))
		assert.Equal(t, "1700000000", q.Get("before"))
		w.Header().Set("Content-Type", "application/json")
		body := `[{"id":1,"name":"Ride A","type":"Ride","sport_type":"Ride","start_date":"2026-01-01T10:00:00Z"},{"id":2,"name":"Run","type":"Run","sport_type":"Run","start_date":"2026-01-01T11:00:00Z"}]`
		if calls == 2 {
			body = `[]`
		}
		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	})

	activities, err := client.List(time.Unix(1600000000, 0), time.Unix(1700000000, 0))
	require.NoError(t, err)
	require.Len(t, activities, 1)
	assert.Equal(t, int64(1), activities[0].ID)
	assert.Equal(t, "Ride A", activities[0].Name)
	assert.Equal(t, time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), activities[0].StartDate)
}

func TestGet(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/activities/42":
			_, err := w.Write([]byte(`{"id":42,"name":"Ride A","type":"Ride","sport_type":"Ride","start_date":"2026-01-01T10:00:00Z","distance":42195,"moving_time":7200,"elapsed_time":7800,"total_elevation_gain":850,"average_speed":5.86}`))
			require.NoError(t, err)
		case "/activities/42/streams":
			assert.Equal(t, "latlng,altitude,time,heartrate,cadence,watts", r.URL.Query().Get("keys"))
			_, err := w.Write([]byte(`{"latlng":{"data":[[43.0,5.0],[43.1,5.1]]},"altitude":{"data":[100,120]},"time":{"data":[0,10]},"heartrate":{"data":[90,91]},"cadence":{"data":[70,null]},"watts":{"data":[200,0]}}`))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	})

	activity, gpxData, err := client.Get(42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), activity.ID)
	assert.Equal(t, 42195.0, activity.DistanceM)
	assert.Equal(t, int64(7200), activity.MovingTimeS)
	assert.NotEmpty(t, gpxData)
	parsed, err := gpx.ParseBytes(gpxData)
	require.NoError(t, err)
	require.Len(t, parsed.Tracks[0].Segments[0].Points, 2)
	const garminNamespace = "http://www.garmin.com/xmlschemas/TrackPointExtension/v1"
	firstExtension, found := parsed.Tracks[0].Segments[0].Points[0].Extensions.GetNode(gpx.NamespaceURL(garminNamespace), "TrackPointExtension")
	require.True(t, found)
	heartRate, found := firstExtension.GetNode("hr")
	require.True(t, found)
	assert.Equal(t, "90", heartRate.Data)
	cadence, found := firstExtension.GetNode("cad")
	require.True(t, found)
	assert.Equal(t, "70", cadence.Data)
	power, found := firstExtension.GetNode("watts")
	require.True(t, found)
	assert.Equal(t, "200", power.Data)

	secondExtension, found := parsed.Tracks[0].Segments[0].Points[1].Extensions.GetNode(gpx.NamespaceURL(garminNamespace), "TrackPointExtension")
	require.True(t, found)
	_, found = secondExtension.GetNode("cad")
	assert.False(t, found)
	power, found = secondExtension.GetNode("watts")
	require.True(t, found)
	assert.Equal(t, "0", power.Data)

	parsedRide, err := (ride.GPXRideParser{}).Parse(bytes.NewReader(gpxData))
	require.NoError(t, err)
	assert.Equal(t, 2, parsedRide.Len())
	heartRateValue, found := parsedRide.HeartRateBpm(1)
	require.True(t, found)
	assert.Equal(t, 91.0, heartRateValue)
	_, found = parsedRide.CadenceRpm(0)
	assert.False(t, found)
	powerValue, found := parsedRide.PowerW(1)
	require.True(t, found)
	assert.Equal(t, 0.0, powerValue)
}
