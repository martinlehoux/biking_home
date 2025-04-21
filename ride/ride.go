package ride

import (
	"github.com/jftuga/geodist"
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
		points[i] = Point{DistanceM: distance, ElevationM: p.Elevation.Value(), Coord: geodist.Coord{Lat: p.Latitude, Lon: p.Longitude}, Timestamp: p.Timestamp}
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

func (r *Ride) ScoreFromKm(start, end float64) float64 {
	i := 0
	j := 0
	for k, p := range r.points {
		if i == 0 && p.DistanceM >= start*1000 {
			i = k
		}
		if j == 0 && p.DistanceM >= end*1000 {
			j = k
			break
		}
	}
	return Score(r.points, i, j)
}

func (r *Ride) ClimbFromDist(startDist, endDist float64) Climb {
	start, end := 0, 0
	for i, p := range r.points {
		if start == 0 && p.DistanceM >= startDist {
			start = i
		}
		if end == 0 && p.DistanceM >= endDist {
			end = i
			break
		}
	}
	return Climb{start: start, end: end, points: r.points[start : end+1]}
}
