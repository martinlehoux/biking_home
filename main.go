package main

import (
	"flag"
	"os"
	"runtime/pprof"

	"github.com/martinlehoux/kagamigo/kcore"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		kcore.Expect(err, "failed to create CPU profile")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
}
