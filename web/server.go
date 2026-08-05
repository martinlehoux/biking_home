package web

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/rides"
	"github.com/martinlehoux/biking_home/strava"
	"github.com/martinlehoux/kagamigo/kcore"
)

const dateFormat = "2006-01-02"

type Server struct {
	db      *sql.DB
	envPath string
	gpxDir  string
	baseURL string

	oauthMu     sync.Mutex
	oauthState  string
	returnToURL string
}

func NewServer(db *sql.DB, envPath, gpxDir, baseURL string) *Server {
	return &Server{db: db, envPath: envPath, gpxDir: gpxDir, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleRides)
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
	items, err := rides.List(s.db)
	if err != nil {
		http.Error(w, "failed to load rides", http.StatusInternalServerError)
		return
	}
	kcore.RenderPage(r.Context(), RidesPage(buildRideViews(items)), w)
}

const minimumDisplayedDistanceM = 10_000

func buildRideViews(items []rides.Ride) []RideView {
	parser := ride.GPXRideParser{}
	views := make([]RideView, 0, len(items))
	for _, item := range items {
		if item.DistanceM < minimumDisplayedDistanceM {
			continue
		}
		view := RideView{Ride: item, Cotacol: "-"}
		parsed, err := ride.ParseFile(parser, item.GPXPath)
		if err != nil {
			slog.Warn("Failed to compute Cotacol", "ride", item.ExternalID, "file", item.GPXPath, "error", err)
		} else {
			score := parsed.DifficultyScore()
			view.Cotacol = formatCotacol(score)
			view.CotacolPer100Km = formatCotacolPer100Km(score, item.DistanceM)
		}
		views = append(views, view)
	}
	return views
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	from, to, err := parseDateRange(r.FormValue("from"), r.FormValue("to"))
	if err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	}
	client, authorized, err := s.stravaClient()
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

	if refreshed, err := client.RefreshIfNeeded(); err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	} else if refreshed {
		if err := s.saveStravaToken(client); err != nil {
			s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
			return
		}
	}
	imported, skipped, err := s.syncRides(client, from, to)
	if err != nil {
		s.renderSyncError(w, r, r.FormValue("from"), r.FormValue("to"), err)
		return
	}
	query := url.Values{}
	query.Set("from", r.FormValue("from"))
	query.Set("to", r.FormValue("to"))
	query.Set("imported", strconv.Itoa(imported))
	query.Set("skipped", strconv.Itoa(skipped))
	http.Redirect(w, r, "/sync?"+query.Encode(), http.StatusSeeOther)
}

func (s *Server) handleStravaLogin(w http.ResponseWriter, r *http.Request) {
	env, err := config.LoadEnv(s.envPath)
	if err != nil {
		http.Error(w, "failed to load configuration", http.StatusInternalServerError)
		return
	}
	clientID := env["STRAVA_CLIENT_ID"]
	clientSecret := env["STRAVA_CLIENT_SECRET"]
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
	redirectURI := s.baseURL + "/strava/callback"
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
	env, err := config.LoadEnv(s.envPath)
	if err != nil {
		http.Error(w, "failed to load configuration", http.StatusInternalServerError)
		return
	}
	redirectURI := s.baseURL + "/strava/callback"
	token, err := strava.ExchangeCode(env["STRAVA_CLIENT_ID"], env["STRAVA_CLIENT_SECRET"], code, redirectURI)
	if err != nil {
		http.Error(w, "failed to exchange Strava authorization code", http.StatusBadGateway)
		return
	}
	if err := config.UpdateEnv(s.envPath, map[string]string{
		"STRAVA_ACCESS_TOKEN":  token.AccessToken,
		"STRAVA_REFRESH_TOKEN": token.RefreshToken,
		"STRAVA_EXPIRES_AT":    strconv.FormatInt(token.ExpiresAt.Unix(), 10),
	}); err != nil {
		http.Error(w, "failed to store Strava token", http.StatusInternalServerError)
		return
	}
	slog.Info("Strava authorization completed")
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (s *Server) syncRides(client *strava.Client, from, to time.Time) (imported, skipped int, err error) {
	activities, err := client.List(from, to)
	if err != nil {
		return 0, 0, err
	}
	if err := os.MkdirAll(s.gpxDir, 0o755); err != nil {
		return 0, 0, fmt.Errorf("create GPX directory: %w", err)
	}
	for _, summary := range activities {
		externalID := fmt.Sprintf("strava:%d", summary.ID)
		_, exists, err := rides.GetByExternalID(s.db, externalID)
		if err != nil {
			return imported, skipped, err
		}
		if exists {
			skipped++
			continue
		}
		activity, gpxData, err := client.Get(summary.ID)
		if err != nil {
			return imported, skipped, err
		}
		gpxPath := filepath.Join(s.gpxDir, fmt.Sprintf("activity_%d.gpx", activity.ID))
		if err := os.WriteFile(gpxPath, gpxData, 0o644); err != nil {
			return imported, skipped, fmt.Errorf("write GPX for activity %d: %w", activity.ID, err)
		}
		activityType := activity.SportType
		if activityType == "" {
			activityType = activity.Type
		}
		if err := rides.Save(s.db, rides.Ride{
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
		}); err != nil {
			return imported, skipped, fmt.Errorf("save activity %d: %w", activity.ID, err)
		}
		imported++
		slog.Info("Imported Strava ride", "activity", activity.ID, "name", activity.Name)
	}
	return imported, skipped, nil
}

func (s *Server) stravaClient() (*strava.Client, bool, error) {
	env, err := config.LoadEnv(s.envPath)
	if err != nil {
		return nil, false, fmt.Errorf("load configuration: %w", err)
	}
	if env["STRAVA_CLIENT_ID"] == "" || env["STRAVA_CLIENT_SECRET"] == "" {
		return nil, false, errors.New("STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET are required")
	}
	if env["STRAVA_ACCESS_TOKEN"] == "" || env["STRAVA_REFRESH_TOKEN"] == "" {
		return nil, false, nil
	}
	var expiresAt time.Time
	if unix, err := strconv.ParseInt(env["STRAVA_EXPIRES_AT"], 10, 64); err == nil && unix != 0 {
		expiresAt = time.Unix(unix, 0)
	}
	return strava.NewClient(env["STRAVA_CLIENT_ID"], env["STRAVA_CLIENT_SECRET"], strava.Token{
		AccessToken:  env["STRAVA_ACCESS_TOKEN"],
		RefreshToken: env["STRAVA_REFRESH_TOKEN"],
		ExpiresAt:    expiresAt,
	}), true, nil
}

func (s *Server) saveStravaToken(client *strava.Client) error {
	token := client.Tokens()
	return config.UpdateEnv(s.envPath, map[string]string{
		"STRAVA_ACCESS_TOKEN":  token.AccessToken,
		"STRAVA_REFRESH_TOKEN": token.RefreshToken,
		"STRAVA_EXPIRES_AT":    strconv.FormatInt(token.ExpiresAt.Unix(), 10),
	})
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
	return fmt.Sprintf("Sync complete: %s imported, %s already stored.", imported, r.URL.Query().Get("skipped"))
}

func safeReturnTo(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/sync"
	}
	return value
}
