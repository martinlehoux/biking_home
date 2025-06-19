package main

import (
	"flag"
	"log/slog"
	"os"
	"runtime/pprof"

	"github.com/martinlehoux/biking_home/importer"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/kagamigo/kcore"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var parser = ride.GPXRideParser{}

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		kcore.Expect(err, "failed to create CPU profile")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	ride19Jan, _ := ride.ParseFile(parser, "examples/activity_18043356988.gpx")
	ride30Mar, _ := ride.ParseFile(parser, "examples/activity_18679866717.gpx")
	ride20Apr, _ := ride.ParseFile(parser, "examples/activity_18880605641.gpx")
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
	i := importer.GarminImporter{}
	i.Run()
}
