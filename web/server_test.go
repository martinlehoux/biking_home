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

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/biking_home/internal/dbtest"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/official_climb"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/rides"
	"github.com/martinlehoux/biking_home/strava"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWebTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db := dbtest.New(t)
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
const officialClimbTrackGPX = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg><trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt><trkpt lat="43.005" lon="5.005"><ele>300</ele></trkpt><trkpt lat="43.01" lon="5.01"><ele>100</ele></trkpt></trkseg></trk></gpx>`

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
	assert.Contains(t, body, `<h1 class="ride-detail-title">Detailed Ride</h1>`)
	assert.NotContains(t, body, `<p class="eyebrow">Ride detail</p>`)
	assert.Contains(t, body, `id="ride-route"`)
	assert.Contains(t, body, `id="ride-profile"`)
	assert.Contains(t, body, `id="ride-profile-chart"`)
	assert.Contains(t, body, `id="ride-map"`)
	assert.Contains(t, body, `class="ride-detail-grid"`)
	assert.Contains(t, body, `class="ride-detail-sidebar"`)
	assert.Contains(t, body, `class="ride-detail-main"`)
	assert.Contains(t, body, `<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js" defer></script>`)
	assert.Contains(t, body, `<script src="/static/ride-detail.js" defer></script>`)
	assert.NotContains(t, body, "pointermove")

	staticRequest := httptest.NewRequest(http.MethodGet, "/static/ride-detail.js", nil)
	staticResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(staticResponse, staticRequest)

	script := staticResponse.Body.String()
	assert.Equal(t, http.StatusOK, staticResponse.Code)
	assert.Equal(t, "text/javascript; charset=utf-8", staticResponse.Header().Get("Content-Type"))
	assert.Contains(t, script, "pointermove")
	assert.Contains(t, script, "circleMarker")
	assert.Contains(t, script, "crossing.passElevationM")
	assert.Contains(t, script, "climbRoute")
	assert.Contains(t, script, "L.polyline")
	assert.Contains(t, script, "zoomToClimb")
	assert.Contains(t, script, "Cotacol")
	assert.Contains(t, script, "cotacolForClimb")
	assert.Contains(t, script, "const labelY = plot.top - 6")
	assert.Contains(t, body, "leaflet@1.9.4/dist/leaflet.js")
	assert.Contains(t, script, "tile.openstreetmap.org/{z}/{x}/{y}.png")
	assert.Contains(t, body, `"type":"FeatureCollection"`)
	assert.Contains(t, body, `"type":"LineString"`)
	assert.Contains(t, body, `"coordinates":[[5,43]`)
}

func TestBuildRideProfileIncludesClimbsAndCrossings(t *testing.T) {
	parsed := ride.FromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.63, Lon: 5.44}},
		make([]time.Time, 3),
	)
	passes := []mountain_pass.MountainPass{{
		Name:      "Pas de Magnan",
		Elevation: 440,
		Coord:     &geodist.Coord{Lat: 43.62, Lon: 5.43},
	}}

	profile := buildRideProfile(parsed, passes, nil, official_climb.DefaultMatchPolicy())

	require.Len(t, profile.Points, 3)
	assert.Equal(t, 1.0, profile.Points[1].DistanceKm)
	assert.Equal(t, 447.0, profile.Points[1].ElevationM)
	require.Len(t, profile.Climbs, 1)
	assert.Equal(t, "Pas de Magnan", profile.Climbs[0].Name)
	assert.Equal(t, 1.0, profile.Climbs[0].TopKm)
	require.Len(t, profile.Crossings, 1)
	assert.Equal(t, "Pas de Magnan", profile.Crossings[0].Name)
	assert.Equal(t, 1.0, profile.Crossings[0].DistanceKm)
}

func TestBuildRideProfileIncludesOfficialClimbMatch(t *testing.T) {
	parsed := ride.FromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.63, Lon: 5.44}},
		make([]time.Time, 3),
	)
	profile := buildRideProfile(parsed, nil, []official_climb.OfficialClimb{{
		ID:         42,
		Name:       "Col de Test",
		StartCoord: parsed.Coord(0),
		EndCoord:   parsed.Coord(1),
	}}, official_climb.DefaultMatchPolicy())

	require.Len(t, profile.Climbs, 1)
	assert.Equal(t, int64(42), profile.Climbs[0].OfficialClimbID)
	assert.Equal(t, "Col de Test", profile.Climbs[0].OfficialName)
	assert.Equal(t, "Col de Test", profile.Climbs[0].Name)
}

func TestBuildRideProfileUsesOfficialClimbBoundaries(t *testing.T) {
	parsed := ride.FromColumns(
		[]float64{0, 1000, 2000},
		[]float64{300, 447, 380},
		[]geodist.Coord{{Lat: 43.61, Lon: 5.42}, {Lat: 43.62, Lon: 5.43}, {Lat: 43.6205, Lon: 5.4305}},
		make([]time.Time, 3),
	)
	profile := buildRideProfile(parsed, nil, []official_climb.OfficialClimb{{
		ID:         42,
		Name:       "Col de Test",
		StartCoord: parsed.Coord(0),
		EndCoord:   parsed.Coord(2),
	}}, official_climb.DefaultMatchPolicy())

	require.Len(t, profile.Climbs, 1)
	assert.Equal(t, 0, profile.Climbs[0].StartIndex)
	assert.Equal(t, 2, profile.Climbs[0].EndIndex)
	assert.Equal(t, 2.0, profile.Climbs[0].EndKm)
	assert.Equal(t, 2.0, profile.Climbs[0].DistanceKm)
	assert.Equal(t, 4.0, profile.Climbs[0].SlopePercent)
	assert.Greater(t, profile.Climbs[0].Cotacol, 0.0)
}

func TestHandlerCreatesOfficialClimbFromRoutePoints(t *testing.T) {
	server, db := newWebTestServer(t)
	routePath := filepath.Join(t.TempDir(), "official-climb.gpx")
	require.NoError(t, os.WriteFile(routePath, []byte(officialClimbTrackGPX), 0o600))
	require.NoError(t, rides.Save(db, rides.Ride{
		ExternalID: "strava:550e8400-e29b-41d4-a716-446655440000",
		GPXPath:    routePath,
		Name:       "Climb Candidate",
		Type:       "Ride",
		StartDate:  time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC),
		DistanceM:  2_000,
	}))
	item, found, err := rides.GetByExternalID(db, "strava:550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.True(t, found)

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rides/%d", item.ID), nil)
	getResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(getResponse, getRequest)
	assert.Equal(t, http.StatusOK, getResponse.Code)
	assert.Contains(t, getResponse.Body.String(), "No official climb matched yet.")
	assert.Contains(t, getResponse.Body.String(), "Select on profile")
	assert.Contains(t, getResponse.Body.String(), "Select on map")
	assert.Contains(t, getResponse.Body.String(), "Climb 1")
	assert.Contains(t, getResponse.Body.String(), "data-climb-next")

	form := url.Values{"name": {"Col de Test"}, "start_index": {"0"}, "end_index": {"1"}}
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/rides/%d/official-climbs", item.ID), strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, fmt.Sprintf("/rides/%d?official_climb=created", item.ID), response.Header().Get("Location"))
	climbs, err := official_climb.List(db)
	require.NoError(t, err)
	require.Len(t, climbs, 1)
	assert.Equal(t, "Col de Test", climbs[0].Name)
	assert.Equal(t, geodist.Coord{Lat: 43.0, Lon: 5.0}, climbs[0].StartCoord)
	assert.Equal(t, geodist.Coord{Lat: 43.005, Lon: 5.005}, climbs[0].EndCoord)

	getRequest = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/rides/%d?official_climb=created", item.ID), nil)
	getResponse = httptest.NewRecorder()
	server.Handler().ServeHTTP(getResponse, getRequest)
	assert.Contains(t, getResponse.Body.String(), "Official climb saved and matched to this ride by coordinates.")
	assert.Contains(t, getResponse.Body.String(), "Official climb matched: Col de Test")
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
