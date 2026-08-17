package web

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/official_climb"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/rides"
	"github.com/martinlehoux/biking_home/strava"
	"github.com/martinlehoux/kagamigo/kcore"
)

const dateFormat = "2006-01-02"

type Server struct {
	db         *sql.DB
	configPath string

	oauthMu     sync.Mutex
	oauthState  string
	returnToURL string
	syncMu      sync.Mutex
	syncActive  bool
}

type syncClient interface {
	List(from, to time.Time) ([]strava.Activity, error)
	Get(id int64) (strava.Activity, []byte, error)
}

func NewServer(db *sql.DB, configPath string) *Server {
	return &Server{db: db, configPath: configPath}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("GET /", s.handleRides)
	mux.HandleFunc("GET /rides/{id}", s.handleRide)
	mux.HandleFunc("POST /rides/{id}/official-climbs", s.handleCreateOfficialClimb)
	mux.HandleFunc("GET /sync", s.handleSyncForm)
	mux.HandleFunc("POST /sync", s.handleSync)
	mux.HandleFunc("GET /strava/login", s.handleStravaLogin)
	mux.HandleFunc("GET /strava/callback", s.handleStravaCallback)
	return kcore.RecoverMiddleware(mux)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleRides(w http.ResponseWriter, r *http.Request) {
	rideSort := parseRideSort(r.URL.Query())
	items, err := s.listRides(rideSort)
	if err != nil {
		slog.Error("Failed to load rides", "sort", rideSort.Column, "descending", rideSort.Descending, "error", err)
		http.Error(w, "failed to load rides", http.StatusInternalServerError)
		return
	}
	views := buildRideViews(items)
	kcore.RenderPage(r.Context(), RidesPage(views, rideSortHeaders(rideSort)), w)
}

func (s *Server) handleRide(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	item, found, err := rides.GetByID(s.db, id)
	if err != nil {
		slog.Error("Failed to load ride", "ride_id", id, "error", err)
		http.Error(w, "failed to load ride", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if item.GPXPath == "" {
		slog.Info("Loaded ride detail without route", "ride_id", id)
		kcore.RenderPage(r.Context(), RideDetailPage(RideDetailView{RideView: buildRideView(item), Notice: rideDetailNotice(r)}), w)
		return
	}
	parsed, err := ride.ParseFile(ride.GPXRideParser{}, item.GPXPath)
	if err != nil {
		slog.Error("Failed to load ride route", "ride_id", id, "file", item.GPXPath, "error", err)
		kcore.RenderPage(r.Context(), RideDetailPage(RideDetailView{
			RideView:   buildRideView(item),
			RouteError: "The recorded route is unavailable.",
			Notice:     rideDetailNotice(r),
		}), w)
		return
	}
	passes, err := mountain_pass.LoadMountainPassesAroundRide(s.db, parsed, rideDetailMountainPassMarginM)
	if err != nil {
		slog.Warn("Failed to load nearby mountain passes for ride detail", "ride_id", id, "error", err)
	}
	officialClimbs, err := official_climb.List(s.db)
	if err != nil {
		slog.Warn("Failed to load official climbs for ride detail", "ride_id", id, "error", err)
	}
	matchPolicy := official_climb.DefaultMatchPolicy()
	appConfig, err := s.loadConfig()
	if err != nil {
		slog.Warn("Failed to load official climb matching configuration", "ride_id", id, "error", err)
	} else {
		matchPolicy.EndpointRadiusM = appConfig.OfficialClimb.MatchRadiusM
	}
	slog.Info("Loaded ride detail", "ride_id", id)
	view := buildRideDetailView(item, parsed, passes, officialClimbs, matchPolicy)
	view.Notice = rideDetailNotice(r)
	kcore.RenderPage(r.Context(), RideDetailPage(view), w)
}

func (s *Server) handleCreateOfficialClimb(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	item, found, err := rides.GetByID(s.db, id)
	if err != nil {
		slog.Error("Failed to load ride for official climb creation", "ride_id", id, "error", err)
		http.Error(w, "failed to load ride", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if item.GPXPath == "" {
		http.Error(w, "a recorded route is required to create an official climb", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	startIndex, err := parseRouteIndex(r.FormValue("start_index"))
	if err != nil {
		http.Error(w, "a valid start route point is required", http.StatusBadRequest)
		return
	}
	endIndex, err := parseRouteIndex(r.FormValue("end_index"))
	if err != nil {
		http.Error(w, "a valid end route point is required", http.StatusBadRequest)
		return
	}
	parsed, err := ride.ParseFile(ride.GPXRideParser{}, item.GPXPath)
	if err != nil {
		slog.Error("Failed to load ride route for official climb creation", "ride_id", id, "file", item.GPXPath, "error", err)
		http.Error(w, "the recorded route is unavailable", http.StatusBadRequest)
		return
	}
	if startIndex >= endIndex || endIndex >= parsed.Len() {
		http.Error(w, "the official climb boundaries must be ordered route points", http.StatusBadRequest)
		return
	}
	created, err := official_climb.Create(s.db, official_climb.OfficialClimb{
		Name:       r.FormValue("name"),
		StartCoord: parsed.Coord(startIndex),
		EndCoord:   parsed.Coord(endIndex),
	})
	if err != nil {
		slog.Warn("Failed to create official climb", "ride_id", id, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slog.Info("Created official climb", "official_climb_id", created.ID, "ride_id", id, "start_index", startIndex, "end_index", endIndex)
	http.Redirect(w, r, rideDetailURL(id)+"?official_climb=created", http.StatusSeeOther)
}

func parseRouteIndex(value string) (int, error) {
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return 0, errors.New("invalid route point index")
	}
	return index, nil
}

func (s *Server) listRides(rideSort RideSort) ([]rides.Ride, error) {
	column, found := rideSort.databaseColumn()
	if !found {
		return rides.List(s.db)
	}
	return rides.ListSorted(s.db, column, rideSort.Descending)
}

const minimumDisplayedDistanceM = 10_000
const rideDetailMountainPassMarginM = 10_000

func buildRideViews(items []rides.Ride) []RideView {
	views := make([]RideView, 0, len(items))
	for _, item := range items {
		if item.DistanceM < minimumDisplayedDistanceM {
			continue
		}
		views = append(views, buildRideView(item))
	}
	return views
}

func buildRideView(item rides.Ride) RideView {
	view := RideView{Ride: item, Cotacol: "-", CotacolPer100Km: "-"}
	cotacol, ready := item.CotacolScore()
	if ready && item.CotacolAlgorithmVersion() == ride.CotacolAlgorithmVersion && item.DistanceM > 0 {
		view.Cotacol = formatCotacol(cotacol)
		view.CotacolPer100Km = formatCotacolPer100Km(cotacol, item.DistanceM)
	}
	return view
}

func (s *Server) handleSyncForm(w http.ResponseWriter, r *http.Request) {
	data := SyncPageData{
		From:    queryOrDefault(r, "from", time.Now().AddDate(0, 0, -30).Format(dateFormat)),
		To:      queryOrDefault(r, "to", time.Now().Format(dateFormat)),
		Notice:  syncNotice(r),
		HasAuth: s.hasStravaToken(),
	}
	if _, _, err := s.stravaClient(); err != nil {
		data.Error = err.Error()
	}
	kcore.RenderPage(r.Context(), SyncPage(data), w)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	from, to, err := parseDateRange(r.FormValue("from"), r.FormValue("to"))
	if err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	}
	appConfig, err := s.loadConfig()
	if err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	}
	client, authorized, err := newStravaClient(appConfig)
	if err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	}
	if !authorized {
		query := url.Values{}
		query.Set("return_to", "/sync?from="+r.FormValue("from")+"&to="+r.FormValue("to"))
		http.Redirect(w, r, "/strava/login?"+query.Encode(), http.StatusFound)
		return
	}
	if !s.beginSync() {
		http.Error(w, "a Strava sync is already in progress", http.StatusConflict)
		return
	}
	defer s.endSync()

	if refreshed, err := client.RefreshIfNeeded(); err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	} else if refreshed {
		if err := s.saveStravaToken(client); err != nil {
			s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming responses are not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/event-stream")
	progress, err := s.syncRides(client, appConfig.Storage.GPXDir, from, to, func(progress SyncProgress) error {
		return writeSyncEvent(w, flusher, "progress", progress)
	})
	if err != nil {
		_ = writeSyncEvent(w, flusher, "error", syncErrorEvent{Message: err.Error(), Progress: progress})
		return
	}
	slog.Info("Completed Strava sync", "from", from.Format(dateFormat), "to", to.Add(-24*time.Hour).Format(dateFormat), "total", progress.Total, "imported", progress.Imported, "skipped", progress.Skipped)
	_ = writeSyncEvent(w, flusher, "complete", progress)
}

func (s *Server) handleStravaLogin(w http.ResponseWriter, r *http.Request) {
	appConfig, err := s.loadConfig()
	if err != nil {
		http.Error(w, "failed to load configuration", http.StatusInternalServerError)
		return
	}
	clientID := appConfig.Strava.ClientID
	clientSecret := appConfig.Strava.ClientSecret
	if clientID == "" || clientSecret == "" {
		http.Error(w, "STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET are required", http.StatusInternalServerError)
		return
	}
	state, err := newOAuthState()
	if err != nil {
		http.Error(w, "failed to start OAuth flow", http.StatusInternalServerError)
		return
	}
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	s.oauthMu.Lock()
	s.oauthState = state
	s.returnToURL = returnTo
	s.oauthMu.Unlock()
	redirectURI := appConfig.Server.PublicURL + "/strava/callback"
	http.Redirect(w, r, strava.AuthorizeURLWithState(clientID, redirectURI, state), http.StatusFound)
}

func (s *Server) handleStravaCallback(w http.ResponseWriter, r *http.Request) {
	if callbackError := r.URL.Query().Get("error"); callbackError != "" {
		http.Error(w, "Strava authorization was denied: "+callbackError, http.StatusBadRequest)
		return
	}
	returnTo, ok := s.consumeOAuthState(r.URL.Query().Get("state"))
	if !ok {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}
	appConfig, err := s.loadConfig()
	if err != nil {
		http.Error(w, "failed to load configuration", http.StatusInternalServerError)
		return
	}
	redirectURI := appConfig.Server.PublicURL + "/strava/callback"
	token, err := strava.ExchangeCode(appConfig.Strava.ClientID, appConfig.Strava.ClientSecret, code, redirectURI)
	if err != nil {
		http.Error(w, "failed to exchange Strava authorization code", http.StatusBadGateway)
		return
	}
	appConfig.Strava.AccessToken = token.AccessToken
	appConfig.Strava.RefreshToken = token.RefreshToken
	appConfig.Strava.ExpiresAt = token.ExpiresAt.Unix()
	if err := config.Save(s.configPath, appConfig); err != nil {
		http.Error(w, "failed to store Strava token", http.StatusInternalServerError)
		return
	}
	slog.Info("Strava authorization completed")
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (s *Server) syncRides(client syncClient, gpxDir string, from, to time.Time, report func(SyncProgress) error) (SyncProgress, error) {
	activities, err := client.List(from, to)
	if err != nil {
		return SyncProgress{}, err
	}
	progress := SyncProgress{Total: len(activities)}
	if err := reportSyncProgress(report, progress); err != nil {
		return progress, err
	}
	slog.Info("Started Strava sync", "from", from.Format(dateFormat), "to", to.Add(-24*time.Hour).Format(dateFormat), "total", progress.Total)
	if err := os.MkdirAll(gpxDir, 0o755); err != nil {
		return progress, fmt.Errorf("create GPX directory: %w", err)
	}
	for _, summary := range activities {
		externalID := fmt.Sprintf("strava:%d", summary.ID)
		_, exists, err := rides.GetByExternalID(s.db, externalID)
		if err != nil {
			return progress, err
		}
		if exists {
			progress.Skipped++
			progress.Completed++
			if err := reportSyncProgress(report, progress); err != nil {
				return progress, err
			}
			continue
		}
		activity, gpxData, err := client.Get(summary.ID)
		if err != nil {
			if errors.Is(err, strava.ErrNoTrackPoints) {
				_ = os.Remove(filepath.Join(gpxDir, fmt.Sprintf("activity_%d.gpx", summary.ID)))
				if saveErr := rides.Save(s.db, rideFromActivity(externalID, "", activity)); saveErr != nil {
					if reportErr := skipSyncActivity(&progress, report, summary, fmt.Errorf("save metadata: %w", saveErr)); reportErr != nil {
						return progress, reportErr
					}
					continue
				}
				progress.Imported++
				progress.Completed++
				if reportErr := reportSyncProgress(report, progress); reportErr != nil {
					return progress, reportErr
				}
				slog.Info("Imported Strava activity without route", "activity", activity.ID, "name", activity.Name)
				continue
			}
			if err := skipSyncActivity(&progress, report, summary, err); err != nil {
				return progress, err
			}
			continue
		}
		gpxPath := filepath.Join(gpxDir, fmt.Sprintf("activity_%d.gpx", activity.ID))
		if err := os.WriteFile(gpxPath, gpxData, 0o644); err != nil {
			_ = os.Remove(gpxPath)
			if reportErr := skipSyncActivity(&progress, report, summary, fmt.Errorf("write GPX: %w", err)); reportErr != nil {
				return progress, reportErr
			}
			continue
		}
		if err := rides.Save(s.db, rideFromActivity(externalID, gpxPath, activity)); err != nil {
			_ = os.Remove(gpxPath)
			if reportErr := skipSyncActivity(&progress, report, summary, fmt.Errorf("save activity: %w", err)); reportErr != nil {
				return progress, reportErr
			}
			continue
		}
		progress.Imported++
		progress.Completed++
		if err := reportSyncProgress(report, progress); err != nil {
			return progress, err
		}
		slog.Info("Imported Strava ride", "activity", activity.ID, "name", activity.Name)
	}
	return progress, nil
}

func rideFromActivity(externalID, gpxPath string, activity strava.Activity) rides.Ride {
	activityType := activity.SportType
	if activityType == "" {
		activityType = activity.Type
	}
	return rides.Ride{
		ExternalID:          externalID,
		GPXPath:             gpxPath,
		Name:                activity.Name,
		Type:                activityType,
		StartDate:           activity.StartDate,
		DistanceM:           activity.DistanceM,
		MovingTimeS:         activity.MovingTimeS,
		ElapsedTimeS:        activity.ElapsedTimeS,
		TotalElevationGainM: activity.TotalElevationGainM,
		AverageSpeedMps:     activity.AverageSpeedMps,
	}
}

func skipSyncActivity(progress *SyncProgress, report func(SyncProgress) error, activity strava.Activity, err error) error {
	progress.Skipped++
	progress.Completed++
	slog.Warn("Skipped Strava activity", "activity", activity.ID, "name", activity.Name, "error", err)
	return reportSyncProgress(report, *progress)
}

func reportSyncProgress(report func(SyncProgress) error, progress SyncProgress) error {
	if report == nil {
		return nil
	}
	return report(progress)
}

func (s *Server) beginSync() bool {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if s.syncActive {
		return false
	}
	s.syncActive = true
	return true
}

func (s *Server) endSync() {
	s.syncMu.Lock()
	s.syncActive = false
	s.syncMu.Unlock()
}

type syncErrorEvent struct {
	Message  string       `json:"message"`
	Progress SyncProgress `json:"progress"`
}

func writeSyncEvent(w http.ResponseWriter, flusher http.Flusher, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (s *Server) stravaClient() (*strava.Client, bool, error) {
	appConfig, err := s.loadConfig()
	if err != nil {
		return nil, false, fmt.Errorf("load configuration: %w", err)
	}
	return newStravaClient(appConfig)
}

func (s *Server) loadConfig() (config.Config, error) {
	return config.Load(s.configPath)
}

func newStravaClient(appConfig config.Config) (*strava.Client, bool, error) {
	if appConfig.Strava.ClientID == "" || appConfig.Strava.ClientSecret == "" {
		return nil, false, errors.New("STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET are required")
	}
	if appConfig.Strava.AccessToken == "" || appConfig.Strava.RefreshToken == "" {
		return nil, false, nil
	}
	var expiresAt time.Time
	if appConfig.Strava.ExpiresAt != 0 {
		expiresAt = time.Unix(appConfig.Strava.ExpiresAt, 0)
	}
	return strava.NewClient(appConfig.Strava.ClientID, appConfig.Strava.ClientSecret, strava.Token{
		AccessToken:  appConfig.Strava.AccessToken,
		RefreshToken: appConfig.Strava.RefreshToken,
		ExpiresAt:    expiresAt,
	}), true, nil
}

func (s *Server) saveStravaToken(client *strava.Client) error {
	appConfig, err := s.loadConfig()
	if err != nil {
		return err
	}
	token := client.Tokens()
	appConfig.Strava.AccessToken = token.AccessToken
	appConfig.Strava.RefreshToken = token.RefreshToken
	appConfig.Strava.ExpiresAt = token.ExpiresAt.Unix()
	return config.Save(s.configPath, appConfig)
}

func (s *Server) hasStravaToken() bool {
	_, authorized, err := s.stravaClient()
	return err == nil && authorized
}

func (s *Server) consumeOAuthState(state string) (string, bool) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	if state == "" || state != s.oauthState {
		return "", false
	}
	returnTo := s.returnToURL
	s.oauthState = ""
	s.returnToURL = ""
	return returnTo, true
}

func (s *Server) renderSyncError(w http.ResponseWriter, r *http.Request, from, to string, err error) {
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	kcore.RenderPage(r.Context(), SyncPage(SyncPageData{From: from, To: to, Error: err.Error(), HasAuth: s.hasStravaToken()}), w)
}

func newOAuthState() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func parseDateRange(from, to string) (time.Time, time.Time, error) {
	start, err := time.Parse(dateFormat, from)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("a valid start date is required")
	}
	end, err := time.Parse(dateFormat, to)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("a valid end date is required")
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, errors.New("the end date must not be before the start date")
	}
	return start.UTC(), end.AddDate(0, 0, 1).UTC(), nil
}

func queryOrDefault(r *http.Request, key, fallback string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return fallback
}

func syncNotice(r *http.Request) string {
	imported := r.URL.Query().Get("imported")
	if imported == "" {
		return ""
	}
	return fmt.Sprintf("Sync complete: %s imported, %s skipped.", imported, r.URL.Query().Get("skipped"))
}

func rideDetailNotice(r *http.Request) string {
	if r.URL.Query().Get("official_climb") == "created" {
		return "Official climb saved and matched to this ride by coordinates."
	}
	return ""
}

func safeReturnTo(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/sync"
	}
	return value
}
