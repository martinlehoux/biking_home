package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/osmpass"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/strava"
	"github.com/martinlehoux/kagamigo/kcore"
	_ "github.com/mattn/go-sqlite3"
)

var (
	download    = flag.Bool("download", false, "download mountain passes into the database")
	resume      = flag.Bool("resume", false, "skip departments already cached on disk")
	importCache = flag.Bool("import-cached", false, "import cached department CSVs into the database")
	fetchOSM    = flag.Bool("fetch-osm", false, "download the France OSM PBF (resumable) into france-latest.osm.pbf")
	extractOSM  = flag.String("extract-osm", "", "extract mountain passes from an OSM PBF file into the database")
	enrich      = flag.Bool("enrich", false, "backfill mountain pass coordinates from OSM data")
	demo        = flag.Bool("demo", false, "run the climb similarity demo")
	chartFile   = flag.String("chart", "", "render a climb/pass chart for a GPX file")
	stravaLogin = flag.Bool("strava-login", false, "authorize the Strava API and store tokens in .env")
	stravaSync  = flag.Bool("strava-sync", false, "download Strava rides as GPX files into rides/")
	stravaSince = flag.Int("strava-since", 30, "days of history to fetch on the first sync (before any .strava-last-sync)")
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	parser      = ride.GPXRideParser{}
)

func main() {
	flag.Parse()
	db, err := sql.Open("sqlite3", "biking_home.db")
	kcore.Expect(err, "failed to open database")
	defer db.Close()
	switch {
	case *download:
		err = mountain_pass.DownloadMountainPasses(db, 5*time.Second, *resume)
		kcore.Expect(err, "failed to download mountain passes")
	case *importCache:
		_, err = mountain_pass.ImportCachedDepartments(db, []string{"06", "13"})
		kcore.Expect(err, "failed to import cached departments")
	case *fetchOSM:
		err = osmpass.FetchFrancePBF("france-latest.osm.pbf")
		kcore.Expect(err, "failed to download France OSM PBF")
	case *extractOSM != "":
		_, err = osmpass.ExtractMountainPasses(context.Background(), *extractOSM, db)
		kcore.Expect(err, "failed to extract mountain passes from OSM")
	case *enrich:
		_, err = osmpass.EnrichMountainPasses(db)
		kcore.Expect(err, "failed to enrich mountain passes")
	case *demo:
		runDemo(db)
	case *stravaLogin:
		runStravaLogin()
	case *stravaSync:
		runStravaSync()
	case *chartFile != "":
		runChart(db, *chartFile)
	}
}

func runStravaLogin() {
	clientID, clientSecret, _ := stravaCredentials(loadEnv())
	if clientID == "" || clientSecret == "" {
		kcore.Expect(errors.New("missing STRAVA_CLIENT_ID / STRAVA_CLIENT_SECRET"), "set both in .env")
	}
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", strava.LoginPort)
	codeCh := make(chan string, 1)
	serverErrCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			serverErrCh <- fmt.Errorf("no code in callback: %s", r.URL.RawQuery)
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}
		codeCh <- code
		fmt.Fprint(w, "Authorization successful — you can close this tab.")
	})
	server := &http.Server{Addr: fmt.Sprintf(":%d", strava.LoginPort), Handler: mux}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()
	defer server.Close()

	slog.Info("Open the authorization URL in your browser", "url", strava.AuthorizeURL(clientID, redirectURI))
	var token strava.Token
	select {
	case code := <-codeCh:
		var err error
		token, err = strava.ExchangeCode(clientID, clientSecret, code, redirectURI)
		kcore.Expect(err, "failed to exchange authorization code")
		slog.Info("Login successful, tokens stored in .env")
	case err := <-serverErrCh:
		kcore.Expect(err, "authorization server failed")
	case <-time.After(5 * time.Minute):
		kcore.Expect(errors.New("timeout"), "no authorization code received")
	}

	validateStrava(clientID, clientSecret, token)
}

func stravaCredentials(env map[string]string) (clientID, clientSecret string, token strava.Token) {
	expiresAt, _ := strconv.ParseInt(env["STRAVA_EXPIRES_AT"], 10, 64)
	return env["STRAVA_CLIENT_ID"], env["STRAVA_CLIENT_SECRET"], strava.Token{
		AccessToken:  env["STRAVA_ACCESS_TOKEN"],
		RefreshToken: env["STRAVA_REFRESH_TOKEN"],
		ExpiresAt:    time.Unix(expiresAt, 0),
	}
}

func validateStrava(clientID, clientSecret string, token strava.Token) {
	client := strava.NewClient(clientID, clientSecret, token)
	if refreshed, err := client.RefreshIfNeeded(); err != nil {
		kcore.Expect(err, "failed to refresh access token")
	} else if refreshed {
		slog.Info("Access token refreshed")
	}
	storeStravaTokens(client.Tokens())
	var athlete struct {
		Firstname string `json:"firstname"`
		Lastname  string `json:"lastname"`
	}
	err := client.GetJSON("/athlete", &athlete)
	kcore.Expect(err, "failed to validate token against /athlete")
	slog.Info("Authenticated as", "athlete", athlete.Firstname+" "+athlete.Lastname)
}

func runStravaSync() {
	clientID, clientSecret, token := stravaCredentials(loadEnv())
	if token.AccessToken == "" || token.RefreshToken == "" {
		kcore.Expect(errors.New("missing STRAVA tokens"), "run -strava-login first")
	}
	client := strava.NewClient(clientID, clientSecret, token)
	if refreshed, err := client.RefreshIfNeeded(); err != nil {
		kcore.Expect(err, "failed to refresh access token")
	} else if refreshed {
		storeStravaTokens(client.Tokens())
		slog.Info("Access token refreshed")
	}

	after := lastStravaSync()
	if after.IsZero() {
		after = time.Now().AddDate(0, 0, -*stravaSince)
	}
	slog.Info("Syncing Strava rides", "since", after.Format("2006-01-02"))
	activities, err := client.ListActivities(after, "Ride")
	kcore.Expect(err, "failed to list Strava activities")
	slog.Info("Rides found", "count", len(activities))

	kcore.Expect(os.MkdirAll("rides", 0o755), "failed to create rides/ directory")
	downloaded := 0
	for _, activity := range activities {
		file := filepath.Join("rides", fmt.Sprintf("activity_%d.gpx", activity.ID))
		if _, err := os.Stat(file); err == nil {
			continue
		}
		latlng, altitude, seconds, err := client.ActivityStreams(activity.ID)
		if err != nil {
			slog.Warn("Failed to fetch streams, skipping", "activity", activity.ID, "error", err)
			continue
		}
		if len(latlng) == 0 || len(altitude) == 0 {
			slog.Warn("Missing latlng or altitude stream, skipping", "activity", activity.ID, "name", activity.Name)
			continue
		}
		err = strava.WriteActivityGPX(file, activity, latlng, altitude, seconds)
		if err != nil {
			slog.Warn("Failed to write GPX, skipping", "activity", activity.ID, "error", err)
			continue
		}
		downloaded++
		slog.Info("Downloaded", "activity", activity.ID, "name", activity.Name, "points", len(latlng))
	}
	updateLastStravaSync(time.Now())
	slog.Info("Sync complete", "downloaded", downloaded, "seen", len(activities))
}

const lastSyncFile = ".strava-last-sync"

func lastStravaSync() time.Time {
	data, err := os.ReadFile(lastSyncFile)
	if err != nil {
		return time.Time{}
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

func updateLastStravaSync(t time.Time) {
	kcore.Expect(os.WriteFile(lastSyncFile, []byte(fmt.Sprintf("%d\n", t.Unix())), 0o644), "failed to write last sync marker")
}

func storeStravaTokens(token strava.Token) {
	updateEnv(map[string]string{
		"STRAVA_ACCESS_TOKEN":  token.AccessToken,
		"STRAVA_REFRESH_TOKEN": token.RefreshToken,
		"STRAVA_EXPIRES_AT":    fmt.Sprintf("%d", token.ExpiresAt.Unix()),
	})
}

func runChart(db *sql.DB, filename string) {
	r, err := ride.ParseFile(parser, filename)
	kcore.Expect(err, "failed to parse ride")
	passes, err := mountain_pass.LoadMountainPasses(db)
	kcore.Expect(err, "failed to load mountain passes")

	climbs := r.AllClimbs()
	for i := range climbs {
		if matched, ok := mountain_pass.MatchClimb(climbs[i], passes, 300, 50); ok {
			climbs[i].Name = matched.Name
		}
	}
	crossings := mountain_pass.DetectCrossings(r, passes, 100, 25)

	output := strings.TrimSuffix(filename, filepath.Ext(filename)) + "-chart.png"
	renderChart(r, climbs, crossings, output)
	slog.Info("Chart written", "file", output)
}

func runDemo(db *sql.DB) {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		kcore.Expect(err, "failed to create CPU profile")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	ride19Jan, _ := ride.ParseFile(parser, "examples/activity_18043356988.gpx")
	ride30Mar, _ := ride.ParseFile(parser, "examples/activity_18679866717.gpx")
	ride20Apr, _ := ride.ParseFile(parser, "examples/activity_18880605641.gpx")
	for _, r := range []*ride.Ride{&ride19Jan, &ride30Mar, &ride20Apr} {
		slog.Info("Ride", "difficulty", r.DifficultyScore())
	}
	passes, passesErr := mountain_pass.LoadMountainPasses(db)
	if passesErr != nil {
		slog.Warn("No mountain passes in database, skipping crossings and climb naming", "error", passesErr)
		passes = nil
	} else {
		rides := []struct {
			name string
			ride ride.Ride
		}{
			{"2022-07-21.Pogacar", func() ride.Ride {
				r, _ := ride.ParseFile(parser, "examples/2022-07-21.Pogacar.gpx")
				return r
			}()},
			{"2023-06-17.AlpesVerdonTour", func() ride.Ride {
				r, _ := ride.ParseFile(parser, "examples/2023-06-17.AlpesVerdonTour.gpx")
				return r
			}()},
			{"2024-12-29.MimetArbois", func() ride.Ride {
				r, _ := ride.ParseFile(parser, "examples/2024-12-29.MimetArbois.gpx")
				return r
			}()},
			{"activity_18043356988", ride19Jan},
			{"activity_18679866717", ride30Mar},
			{"activity_18880605641", ride20Apr},
		}
		for _, entry := range rides {
			crossings := mountain_pass.DetectCrossings(entry.ride, passes, 100, 25)
			if len(crossings) > 0 {
				slog.Info("Crossings", "ride", entry.name, "count", len(crossings))
				for _, crossing := range crossings {
					slog.Info("Crossing", "pass", crossing.String())
				}
			}
		}
	}

	nameClimbs := func(climbs []ride.Climb) {
		for i := range climbs {
			if matched, ok := mountain_pass.MatchClimb(climbs[i], passes, 300, 50); ok {
				climbs[i].Name = matched.Name
			}
		}
	}

	verdon, _ := ride.ParseFile(parser, "examples/2023-06-17.AlpesVerdonTour.gpx")
	verdonClimbs := verdon.AllClimbs()
	nameClimbs(verdonClimbs)
	for _, climb := range verdonClimbs {
		slog.Info("Climb", "climb", climb, "elevation", climb.Top().ElevationM, "duration", climb.Duration(), "speed", climb.Speed()*3.6)
	}

	index := ride.NewFlatRideClimbsIndex(30)
	index.Insert(ride19Jan)
	index.Insert(ride30Mar)
	ride20AprilClimbs := ride20Apr.AllClimbs()
	nameClimbs(ride20AprilClimbs)
	for _, climb := range ride20AprilClimbs {
		slog.Info("Climb", "climb", climb, "duration", climb.Duration(), "speed", climb.Speed()*3.6)
		if len(index.Similar(ride20Apr, climb)) > 0 {
			for _, similar := range index.Similar(ride20Apr, climb) {
				slog.Info("Similar", "climb", similar, "duration", similar.Duration(), "speed", similar.Speed()*3.6)
			}
		}
	}
	ride30MarchClimbs := ride30Mar.AllClimbs()
	nameClimbs(ride30MarchClimbs)
	for _, climb := range ride30MarchClimbs {
		slog.Info("Climb", "climb", climb, "duration", climb.Duration(), "speed", climb.Speed()*3.6)
	}
	ride.PlotScore(&ride30Mar, 61.2, 66.7, "Ride 30 Mar.png")
}
