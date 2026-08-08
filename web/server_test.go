package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/biking_home/rides"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWebTestServer(t *testing.T) (*Server, *sql.DB) {
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
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	appConfig := config.Default()
	appConfig.Strava.ClientID = "123"
	appConfig.Strava.ClientSecret = "secret"
	require.NoError(t, config.Save(configPath, appConfig))
	return NewServer(db, configPath), db
}

func testGPXPath(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(`<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg><trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt><trkpt lat="43.001" lon="5.001"><ele>200</ele></trkpt></trkseg></trk></gpx>`), 0o600))
	return path
}

func TestHandlerRendersRidesPage(t *testing.T) {
	server, db := newWebTestServer(t)
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:1234",
		GPXPath:    testGPXPath(t, "long.gpx"),
		Name:       "Long Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}))
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:5678",
		GPXPath:    testGPXPath(t, "short.gpx"),
		Name:       "Short Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 2, 7, 0, 0, 0, time.UTC),
		DistanceM:  9_999,
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "All rides")
	assert.Contains(t, response.Body.String(), "Long Ride")
	assert.NotContains(t, response.Body.String(), "Short Ride")
	assert.Contains(t, response.Body.String(), "Cotacol")
	assert.Contains(t, response.Body.String(), "Cotacol / 100 km")
}

func TestSyncPageRequestsAuthorization(t *testing.T) {
	server, _ := newWebTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/sync", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Authorize Strava")
}

func TestSyncRedirectsToOAuthWhenUnauthenticated(t *testing.T) {
	server, _ := newWebTestServer(t)
	form := url.Values{"from": {"2026-08-01"}, "to": {"2026-08-04"}}
	req := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusFound, response.Code)
	location, err := url.Parse(response.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/strava/login", location.Path)
	assert.Equal(t, "/sync?from=2026-08-01&to=2026-08-04", location.Query().Get("return_to"))
}

func TestStravaLoginRedirectsToAuthorize(t *testing.T) {
	server, _ := newWebTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/strava/login?return_to=/sync", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusFound, response.Code)
	location, err := url.Parse(response.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "www.strava.com", location.Host)
	assert.Equal(t, "/oauth/authorize", location.Path)
	assert.Equal(t, "activity:read_all", location.Query().Get("scope"))
	assert.Equal(t, "http://localhost:8080/strava/callback", location.Query().Get("redirect_uri"))
	assert.NotEmpty(t, location.Query().Get("state"))
}

func TestParseDateRangeMakesEndInclusive(t *testing.T) {
	from, to, err := parseDateRange("2026-08-01", "2026-08-04")
	require.NoError(t, err)
	assert.Equal(t, "2026-08-01T00:00:00Z", from.Format("2006-01-02T15:04:05Z07:00"))
	assert.Equal(t, "2026-08-05T00:00:00Z", to.Format("2006-01-02T15:04:05Z07:00"))
}

func TestFormatCotacolPer100Km(t *testing.T) {
	assert.Equal(t, "20.0", formatCotacolPer100Km(2, 10_000))
	assert.Equal(t, "-", formatCotacolPer100Km(2, 0))
}

func TestParseRideSort(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		column     string
		descending bool
	}{
		{name: "default", query: "", column: rideSortStarted, descending: true},
		{name: "ascending distance", query: "sort=distance&dir=asc", column: rideSortDistance, descending: false},
		{name: "invalid values", query: "sort=unknown&dir=sideways", column: rideSortStarted, descending: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := url.ParseQuery(tt.query)
			require.NoError(t, err)
			got := parseRideSort(query)
			assert.Equal(t, tt.column, got.Column)
			assert.Equal(t, tt.descending, got.Descending)
		})
	}
}

func TestRideSortHeadersToggleDirection(t *testing.T) {
	headers := rideSortHeaders(RideSort{Column: rideSortDistance, Descending: true})
	require.Len(t, headers, 7)
	assert.Equal(t, "/?dir=asc&sort=distance", headers[2].URL)
	assert.Equal(t, "descending", headers[2].AriaSort)
	assert.Contains(t, headers[2].Class, "active")
	assert.Equal(t, "/?dir=asc&sort=name", headers[0].URL)

	headers = rideSortHeaders(RideSort{Column: rideSortDistance, Descending: false})
	assert.Equal(t, "/?dir=desc&sort=distance", headers[2].URL)
	assert.Equal(t, "ascending", headers[2].AriaSort)
}

func TestSortRideViewsByCotacolKeepsMissingLast(t *testing.T) {
	items := []RideView{
		{cotacolScore: 4, cotacolReady: true},
		{cotacolScore: 1, cotacolReady: true},
		{cotacolReady: false},
	}

	sortRideViews(items, RideSort{Column: rideSortCotacol})
	assert.Equal(t, 1.0, items[0].cotacolScore)
	assert.Equal(t, 4.0, items[1].cotacolScore)
	assert.False(t, items[2].cotacolReady)

	sortRideViews(items, RideSort{Column: rideSortCotacol, Descending: true})
	assert.Equal(t, 4.0, items[0].cotacolScore)
	assert.Equal(t, 1.0, items[1].cotacolScore)
	assert.False(t, items[2].cotacolReady)
}

func TestHandlerSortsRidesByDistance(t *testing.T) {
	server, db := newWebTestServer(t)
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:550e8400-e29b-41d4-a716-446655440000",
		GPXPath:    testGPXPath(t, "near.gpx"),
		Name:       "Near Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}))
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:6ba7b810-9dad-41d1-80b4-00c04fd430c8",
		GPXPath:    testGPXPath(t, "far.gpx"),
		Name:       "Far Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 2, 7, 0, 0, 0, time.UTC),
		DistanceM:  40_000,
	}))

	req := httptest.NewRequest(http.MethodGet, "/?sort=distance&dir=asc", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Less(t, strings.Index(body, "Near Ride"), strings.Index(body, "Far Ride"))
	assert.Contains(t, body, "dir=desc&amp;sort=distance")
}
