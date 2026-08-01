package main

import (
	"database/sql"
	"flag"
	"log/slog"
	"os"
	"runtime/pprof"
	"time"

	mountainpass "github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/kagamigo/kcore"
	_ "github.com/mattn/go-sqlite3"
)

var (
	download   = flag.Bool("download", false, "download mountain passes into the database")
	resume     = flag.Bool("resume", false, "skip departments already cached on disk")
	demo       = flag.Bool("demo", false, "run the climb similarity demo")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	parser     = ride.GPXRideParser{}
)

func main() {
	flag.Parse()
	if *download {
		runDownload()
		return
	}
	if *demo {
		runDemo()
	}
}

func runDownload() {
	db, err := sql.Open("sqlite3", "biking_home.db")
	kcore.Expect(err, "failed to open database")
	defer db.Close()
	err = mountainpass.DownloadMountainPasses(db, 5*time.Second, *resume)
	kcore.Expect(err, "failed to download mountain passes")
}

func runDemo() {
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
	index := ride.NewFlatRideClimbsIndex(30)
	index.Insert(ride19Jan)
	index.Insert(ride30Mar)
	for _, climb := range ride20Apr.AllClimbs() {
		slog.Info("Climb", "climb", climb, "duration", climb.Duration(), "speed", climb.Speed()*3.6)
		if len(index.Similar(ride20Apr, climb)) > 0 {
			for _, similar := range index.Similar(ride20Apr, climb) {
				slog.Info("Similar", "climb", similar, "duration", similar.Duration(), "speed", similar.Speed()*3.6)
			}
		}
	}
	for _, climb := range ride30Mar.AllClimbs() {
		slog.Info("Climb", "climb", climb, "duration", climb.Duration(), "speed", climb.Speed()*3.6)
	}
	ride.PlotScore(&ride30Mar, 61.2, 66.7, "Ride 30 Mar.png")
}
