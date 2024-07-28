package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/pprof"
	"slices"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/olekukonko/tablewriter"
	"github.com/tkrajina/gpxgo/gpx"
)

var ErrAssert = errors.New("assertion error")

func Assert(cond bool, msg string) {
	if !cond {
		err := ErrAssert
		if msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		panic(err)
	}
}

func Score(points []Point, start int, end int) float64 {
	Assert(end > start, "no points for score")
	distance := points[end].distance - points[start].distance
	if distance == 0 {
		return 0
	}
	// TODO: Expect NullableFloat64
	dElevation := points[end].Elevation.Value() - points[start].Elevation.Value()

	return math.Abs(dElevation) * dElevation / distance * 100.0 * 100.0 / 1000.0
}

func Category(score float64) string {
	switch {
	case score < 35:
		return "NO"
	case score < 80:
		return "Cat 4"
	case score < 180:
		return "Cat 3"
	case score < 250:
		return "Cat 2"
	case score < 600:
		return "Cat 1"
	default:
		return "HC"
	}
}

type Climb struct {
	start int
	end   int
}

type Point struct {
	gpx.GPXPoint
	distance float64
}

type Ride struct {
	points []Point
}

func ParseGPX(r io.Reader) Ride {
	content, err := gpx.Parse(r)
	kcore.Expect(err, "failed to parse GPX")
	segment := content.Tracks[0].Segments[0]
	points := make([]Point, len(segment.Points))
	distance := 0.0
	for i, p := range segment.Points {
		if i != 0 {
			distance += p.Distance2D(&segment.Points[i-1])
		}
		Assert(i == 0 || distance > 0, "zero distance")
		points[i] = Point{p, distance}
	}
	return Ride{points}
}

func BestUpHill(points []Point, start int, end int) Climb {
	Assert(end > start, "empty points")
	bestScore := Score(points, start, end)
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(points, i, end)
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := end; i > bestStart; i-- {
		score := Score(points, bestStart, i)
		if score > bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	return Climb{bestStart, bestEnd}
}

func BestDownHill(points []Point, start int, end int) Climb {
	Assert(len(points) > 0, "empty points")
	bestScore := Score(points, start, end)
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(points, i, end)
		if score < bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := end; i > bestStart; i-- {
		score := Score(points, bestStart, i)
		if score < bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	return Climb{bestStart, bestEnd}
}

func FindAllClimbs(points []Point, start int, end int) []Climb {
	fmt.Printf("Searching climbs between %.1fkm and %.1fkm\n", points[start].distance/1000, points[end].distance/1000)
	climbs := []Climb{}
	climb := BestUpHill(points, start, end)
	descent := BestDownHill(points, start, end)
	Assert(climb.start < climb.end, "empty climb")

	if Score(points, climb.start, climb.end) < 35 {
		if Score(points, descent.start, descent.end) < -35 {
			if descent.start > start {
				climbs = append(climbs, FindAllClimbs(points, start, descent.start)...)
			}
			if end > descent.end {
				climbs = append(climbs, FindAllClimbs(points, descent.end, end)...)
			}
		}
	} else {
		climbs = append(climbs, climb)
		fmt.Printf("Found climb between %.1fkm and %.1fkm\n", points[climb.start].distance/1000, points[climb.end].distance/1000)
		if climb.start > start {
			climbs = append(climbs, FindAllClimbs(points, start, climb.start)...)
		}
		if end > climb.end {
			climbs = append(climbs, FindAllClimbs(points, climb.end, end)...)
		}
	}

	return climbs
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		kcore.Expect(err, "failed to create CPU profile")
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	gpxFile, err := os.Open("examples/2022-07-21.Pogacar.gpx")
	kcore.Expect(err, "failed to open GPX file")
	ride := ParseGPX(gpxFile)
	Assert(len(ride.points) > 0, "no points in ride")
	fmt.Printf("Ride loaded:\t%.1fkm\t%d points\n", ride.points[len(ride.points)-1].distance/1000, len(ride.points))
	climbs := FindAllClimbs(ride.points, 0, len(ride.points)-1)
	slices.SortFunc(climbs, func(i, j Climb) int { return i.start - j.start })
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Score", "Distance", "Category", "Slope", "From", "To", "Point span"})
	for _, climb := range climbs {
		table.Append([]string{fmt.Sprintf("%d", int(Score(ride.points, climb.start, climb.end))), fmt.Sprintf("%.1fkm", (ride.points[climb.end].distance-ride.points[climb.start].distance)/1000), Category(Score(ride.points, climb.start, climb.end)), fmt.Sprintf("%.1f%%", (ride.points[climb.end].Elevation.Value()-ride.points[climb.start].Elevation.Value())/(ride.points[climb.end].distance-ride.points[climb.start].distance)*100), fmt.Sprintf("%.1fkm", ride.points[climb.start].distance/1000), fmt.Sprintf("%.1fkm", ride.points[climb.end].distance/1000), fmt.Sprintf("%d", climb.end-climb.start)})
	}
	table.Render()
	charts.NewLine()
	scatter := charts.NewLine()
	values := make([]opts.LineData, len(ride.points))
	for i, p := range ride.points {
		values[i] = opts.LineData{Value: []any{i, p.distance}}
	}
	scatter.AddSeries("distance", values)
	f, err := os.Create("scatter.html")
	kcore.Expect(err, "failed to create scatter plot")
	defer f.Close()
	scatter.Render(f)
}
