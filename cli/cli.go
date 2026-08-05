package cli

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"path/filepath"
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
	chartFile   = flag.String("chart", "", "render a climb/pass chart for a GPX file")
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
