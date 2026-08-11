package cli

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"time"

	"github.com/martinlehoux/biking_home/config"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/osmpass"
	"github.com/martinlehoux/biking_home/rides"
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
	backfill    = flag.Bool("backfill", false, "recompute all stored ride values")
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
	case *backfill:
		count, err := rides.Backfill(db)
		kcore.Expect(err, "failed to backfill ride values")
		slog.Info("Backfilled ride values", "rides", count)
	default:
		runServer(db, configPath, appConfig)
	}
}

func runServer(db *sql.DB, configPath string, appConfig config.Config) {
	server := web.NewServer(db, configPath)
	slog.Info("Starting web server", "address", appConfig.Server.PublicURL)
	kcore.Expect(server.ListenAndServe(appConfig.Server.Address), "web server stopped")
}
