package web

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
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
	"github.com/martinlehoux/biking_home/strava"
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

type fakeSyncClient struct {
	activities []strava.Activity
	results    map[int64]fakeSyncResult
}

type fakeSyncResult struct {
	activity strava.Activity
	data     []byte
	err      error
}

func (c fakeSyncClient) List(time.Time, time.Time) ([]strava.Activity, error) {
	return c.activities, nil
}

func (c fakeSyncClient) Get(id int64) (strava.Activity, []byte, error) {
	result, ok := c.results[id]
	if !ok {
		return strava.Activity{}, nil, fmt.Errorf("missing fake activity %d", id)
	}
	return result.activity, result.data, result.err
}

const emptyTrackGPX = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg></trkseg></trk></gpx>`
const validTrackGPX = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg><trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt><trkpt lat="43.001" lon="5.001"><ele>200</ele></trkpt></trkseg></trk></gpx>`

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
	assert.Contains(t, response.Body.String(), `href="/rides/1"`)
}

func TestHandlerRendersRideDetailWithEmbeddedRoute(t *testing.T) {
	server, db := newWebTestServer(t)
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:550e8400-e29b-41d4-a716-446655440000",
		GPXPath:    testGPXPath(t, "detail.gpx"),
		Name:       "Detailed Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}))
	item, found, err := rides.GetByExternalID(db, "strava:550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.True(t, found)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rides/%d", item.ID), nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, body, "Detailed Ride")
	assert.Contains(t, body, `id="ride-route"`)
	assert.Contains(t, body, `id="ride-map"`)
	assert.Contains(t, body, "leaflet@1.9.4/dist/leaflet.js")
	assert.Contains(t, body, "tile.openstreetmap.org/{z}/{x}/{y}.png")
	assert.Contains(t, body, `"type":"FeatureCollection"`)
	assert.Contains(t, body, `"type":"LineString"`)
	assert.Contains(t, body, `"coordinates":[[5,43]`)
}

func TestHandlerRendersRideDetailWithoutRoute(t *testing.T) {
	server, db := newWebTestServer(t)
	item := rides.Ride{
		ExternalID: "strava:14701658670",
		Name:       "Indoor Ride",
		Type:       "VirtualRide",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}
	require.NoError(t, rides.Save(db, item))
	stored, found, err := rides.GetByExternalID(db, item.ExternalID)
	require.NoError(t, err)
	require.True(t, found)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rides/%d", stored.ID), nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, body, "Indoor Ride")
	assert.NotContains(t, body, "The recorded route is unavailable.")
	assert.NotContains(t, body, `id="ride-map"`)
}

func TestHandlerReturnsNotFoundForUnknownRide(t *testing.T) {
	server, _ := newWebTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/rides/999999", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestHandlerShowsErrorForUnavailableRideRoute(t *testing.T) {
	server, db := newWebTestServer(t)
	_, err := db.Exec(`
		INSERT INTO rides (external_id, gpx_path, name, type, start_date, distance_m, moving_time_s, elapsed_time_s, total_elevation_gain_m, average_speed_mps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "strava:6ba7b810-9dad-41d1-80b4-00c04fd430c8", "missing.gpx", "Unavailable Route", "Ride", "2026-08-01T07:00:00Z", 20_000, 0, 0, 0, 0)
	require.NoError(t, err)
	item, found, err := rides.GetByExternalID(db, "strava:6ba7b810-9dad-41d1-80b4-00c04fd430c8")
	require.NoError(t, err)
	require.True(t, found)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rides/%d", item.ID), nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, body, "Unavailable Route")
	assert.Contains(t, body, "The recorded route is unavailable.")
	assert.NotContains(t, body, `id="ride-route"`)
}

func TestSyncPageRequestsAuthorization(t *testing.T) {
	server, _ := newWebTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/sync", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Authorize Strava")
}

func TestSyncPageIncludesProgressUIWhenAuthorized(t *testing.T) {
	server, _ := newWebTestServer(t)
	appConfig, err := config.Load(server.configPath)
	require.NoError(t, err)
	appConfig.Strava.AccessToken = "access"
	appConfig.Strava.RefreshToken = "refresh"
	appConfig.Strava.ExpiresAt = time.Now().Add(time.Hour).Unix()
	require.NoError(t, config.Save(server.configPath, appConfig))

	req := httptest.NewRequest(http.MethodGet, "/sync", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, body, `id="sync-form"`)
	assert.Contains(t, body, `id="sync-progress"`)
	assert.Contains(t, body, `<progress id="sync-progress-bar"`)
	assert.Contains(t, body, `text/event-stream`)
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

func TestSyncParsesMultipartDateRange(t *testing.T) {
	server, _ := newWebTestServer(t)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("from", "2026-08-11"))
	require.NoError(t, form.WriteField("to", "2026-08-01"))
	require.NoError(t, form.Close())

	req := httptest.NewRequest(http.MethodPost, "/sync", &body)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "the end date must not be before the start date\n", response.Body.String())
	assert.NotContains(t, response.Body.String(), "<!doctype html>")
}

func TestSyncRidesContinuesAfterInvalidActivity(t *testing.T) {
	server, db := newWebTestServer(t)
	invalidID := int64(14701658670)
	validID := int64(14701658671)
	startDate := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)
	client := fakeSyncClient{
		activities: []strava.Activity{
			{ID: invalidID, Name: "Empty activity", Type: "Ride", StartDate: startDate},
			{ID: validID, Name: "Valid activity", Type: "Ride", StartDate: startDate},
		},
		results: map[int64]fakeSyncResult{
			invalidID: {activity: strava.Activity{ID: invalidID, Name: "Empty activity", Type: "Ride", StartDate: startDate}, data: []byte(emptyTrackGPX)},
			validID:   {activity: strava.Activity{ID: validID, Name: "Valid activity", Type: "Ride", StartDate: startDate}, data: []byte(validTrackGPX)},
		},
	}
	var updates []SyncProgress
	gpxDir := t.TempDir()

	progress, err := server.syncRides(client, gpxDir, startDate, startDate.AddDate(0, 0, 1), func(progress SyncProgress) error {
		updates = append(updates, progress)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, SyncProgress{Total: 2, Completed: 2, Imported: 1, Skipped: 1}, progress)
	assert.Len(t, updates, 3)
	_, found, err := rides.GetByExternalID(db, "strava:14701658670")
	require.NoError(t, err)
	assert.False(t, found)
	_, found, err = rides.GetByExternalID(db, "strava:14701658671")
	require.NoError(t, err)
	assert.True(t, found)
	assert.NoFileExists(t, filepath.Join(gpxDir, "activity_14701658670.gpx"))
}

func TestSyncRidesSavesActivityWithoutRoute(t *testing.T) {
	server, db := newWebTestServer(t)
	indoorID := int64(14701658670)
	validID := int64(14701658671)
	startDate := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)
	client := fakeSyncClient{
		activities: []strava.Activity{
			{ID: indoorID, Name: "Indoor activity", Type: "VirtualRide", StartDate: startDate, DistanceM: 20_000},
			{ID: validID, Name: "Valid activity", Type: "Ride", StartDate: startDate},
		},
		results: map[int64]fakeSyncResult{
			indoorID: {activity: strava.Activity{ID: indoorID, Name: "Indoor activity", Type: "VirtualRide", StartDate: startDate, DistanceM: 20_000}, err: strava.ErrNoTrackPoints},
			validID:  {activity: strava.Activity{ID: validID, Name: "Valid activity", Type: "Ride", StartDate: startDate}, data: []byte(validTrackGPX)},
		},
	}
	gpxDir := t.TempDir()
	staleGPXPath := filepath.Join(gpxDir, "activity_14701658670.gpx")
	require.NoError(t, os.WriteFile(staleGPXPath, []byte(emptyTrackGPX), 0o600))

	progress, err := server.syncRides(client, gpxDir, startDate, startDate.AddDate(0, 0, 1), nil)

	require.NoError(t, err)
	assert.Equal(t, SyncProgress{Total: 2, Completed: 2, Imported: 2}, progress)
	indoor, found, err := rides.GetByExternalID(db, "strava:14701658670")
	require.NoError(t, err)
	require.True(t, found)
	assert.Empty(t, indoor.GPXPath)
	_, ready := indoor.CotacolScore()
	assert.False(t, ready)
	assert.NoFileExists(t, staleGPXPath)
	_, found, err = rides.GetByExternalID(db, "strava:14701658671")
	require.NoError(t, err)
	assert.True(t, found)
}

func TestSyncRejectsConcurrentImport(t *testing.T) {
	server, _ := newWebTestServer(t)
	appConfig, err := config.Load(server.configPath)
	require.NoError(t, err)
	appConfig.Strava.AccessToken = "access"
	appConfig.Strava.RefreshToken = "refresh"
	appConfig.Strava.ExpiresAt = time.Now().Add(time.Hour).Unix()
	require.NoError(t, config.Save(server.configPath, appConfig))
	require.True(t, server.beginSync())
	defer server.endSync()

	form := url.Values{"from": {"2026-08-01"}, "to": {"2026-08-04"}}
	req := httptest.NewRequest(http.MethodPost, "/sync", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, req)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Contains(t, response.Body.String(), "already in progress")
}

func TestWriteSyncEvent(t *testing.T) {
	response := httptest.NewRecorder()
	progress := SyncProgress{Total: 2, Completed: 1, Imported: 1}

	require.NoError(t, writeSyncEvent(response, response, "progress", progress))
	require.NoError(t, writeSyncEvent(response, response, "complete", progress))

	body := response.Body.String()
	assert.Contains(t, body, "event: progress\ndata: {\"total\":2,\"completed\":1,\"imported\":1,\"skipped\":0}\n\n")
	assert.Contains(t, body, "event: complete\ndata: {\"total\":2,\"completed\":1,\"imported\":1,\"skipped\":0}\n\n")
	assert.Less(t, strings.Index(body, "event: progress"), strings.Index(body, "event: complete"))
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

func TestBuildRideViewsUsesStoredCotacol(t *testing.T) {
	_, db := newWebTestServer(t)
	ride := rides.Ride{
		ExternalID: "strava:550e8400-e29b-41d4-a716-446655440000",
		GPXPath:    testGPXPath(t, "stored.gpx"),
		Name:       "Stored Ride",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}
	require.NoError(t, rides.Save(db, ride))
	_, err := db.Exec("UPDATE rides SET gpx_path = ? WHERE external_id = ?", "missing.gpx", ride.ExternalID)
	require.NoError(t, err)
	items, err := rides.List(db)
	require.NoError(t, err)

	views := buildRideViews(items)
	require.Len(t, views, 1)
	assert.NotEqual(t, "-", views[0].Cotacol)
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

func TestHandlerSortsRidesByCotacol(t *testing.T) {
	server, db := newWebTestServer(t)
	first := rides.Ride{
		ExternalID: "strava:550e8400-e29b-41d4-a716-446655440000",
		GPXPath:    testGPXPath(t, "first.gpx"),
		Name:       "Low Cotacol",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  20_000,
	}
	second := rides.Ride{
		ExternalID: "strava:6ba7b810-9dad-41d1-80b4-00c04fd430c8",
		GPXPath:    testGPXPath(t, "second.gpx"),
		Name:       "High Cotacol",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 2, 7, 0, 0, 0, time.UTC),
		DistanceM:  40_000,
	}
	require.NoError(t, rides.Save(db, first))
	require.NoError(t, rides.Save(db, second))
	_, err := db.Exec(`
		UPDATE rides
		SET cotacol_score = CASE external_id WHEN ? THEN 1 ELSE 4 END
	`, first.ExternalID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/?sort=cotacol&dir=asc", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)

	body := response.Body.String()
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Less(t, strings.Index(body, "Low Cotacol"), strings.Index(body, "High Cotacol"))
}
