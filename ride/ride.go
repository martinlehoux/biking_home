package ride

import (
	"fmt"

	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
)

type Ride struct {
	points []Point
}

func (r *Ride) check() {
	kcore.Assert(len(r.points) > 0, "no points in ride")
}

func FromGPX(content *gpx.GPX) Ride {
	segment := content.Tracks[0].Segments[0]
	points := make([]Point, len(segment.Points))
	distance := 0.0
	for i, p := range segment.Points {
		if i != 0 {
			distance += p.Distance2D(&segment.Points[i-1])
		}
		kcore.Assert(i == 0 || distance > 0, "zero distance")
		kcore.Assert(p.Elevation.NotNull(), "points without elevation")
		points[i] = Point{distance: distance, elevation: p.Elevation.Value()}
	}
	ride := Ride{points}
	ride.check()
	return ride
}

func FromPoints(points []Point) Ride {
	ride := Ride{points}
	ride.check()
	return ride
}

func (r *Ride) String(climb Climb) string {
	start := r.points[climb.start]
	end := r.points[climb.end]
	score := Score(r.points, climb.start, climb.end)
	return fmt.Sprintf("%.1fkm-%.1fkm: %.1fkm at %.1f%% (%d pts - %s)", start.distance/1000, end.distance/1000, (end.distance-start.distance)/1000, Slope(start, end)*100, int(score), Category(score))
}
