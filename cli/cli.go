package cli

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/martinlehoux/biking_home/chart"
	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/osmpass"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/web"
	"github.com/martinlehoux/kagamigo/kcore"
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
	cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
	parser      = ride.GPXRideParser{}
)

func Run(db *sql.DB, configPath string, appConfig config.Config) {
	switch {
	case *download:
		err := mountain_pass.DownloadMountainPasses(db, 5*time.Second, *resume)
		kcore.Expect(err, "failed to download mountain passes")
	case *importCache:
		_, err := mountain_pass.ImportCachedDepartments(db, []string{"06", "13"})
		kcore.Expect(err, "failed to import cached departments")
	case *fetchOSM:
		err := osmpass.FetchFrancePBF("france-latest.osm.pbf")
		kcore.Expect(err, "failed to download France OSM PBF")
	case *extractOSM != "":
		_, err := osmpass.ExtractMountainPasses(context.Background(), *extractOSM, db)
		kcore.Expect(err, "failed to extract mountain passes from OSM")
	case *enrich:
		_, err := osmpass.EnrichMountainPasses(db)
		kcore.Expect(err, "failed to enrich mountain passes")
	case *demo:
		runDemo(db)
	case *chartFile != "":
		runChart(db, *chartFile)
	default:
		runServer(db, configPath, appConfig)
	}
}

func runServer(db *sql.DB, configPath string, appConfig config.Config) {
	server := web.NewServer(db, configPath)
	slog.Info("Starting web server", "address", appConfig.Server.PublicURL)
	kcore.Expect(server.ListenAndServe(appConfig.Server.Address), "web server stopped")
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
	chart.Render(r, climbs, crossings, output)
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
