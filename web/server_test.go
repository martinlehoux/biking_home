package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestHandlerRendersRidesPage(t *testing.T) {
	server, db := newWebTestServer(t)
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:1234",
		GPXPath:    "missing.gpx",
		Name:       "Long Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}))
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:5678",
		GPXPath:    "short.gpx",
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
