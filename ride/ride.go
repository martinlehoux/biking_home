package ride

import (
	"github.com/jftuga/geodist"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
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
	return Climb{rideStart: start, rideEnd: end, points: r.points[start : end+1]}
}

func Plot(r *Ride, outputFile string) {
	r.check()

	pts := make(plotter.XYs, len(r.points))
	for i, p := range r.points {
		pts[i].X = p.DistanceM / 1000 // Convert distance to kilometers
		pts[i].Y = p.ElevationM
	}

	p := plot.New()
	p.Title.Text = "Ride Elevation Profile"
	p.X.Label.Text = "Distance (km)"
	p.Y.Label.Text = "Elevation (m)"

	line, err := plotter.NewLine(pts)
	kcore.Expect(err, "failed to create line plot")
	p.Add(line)

	err = p.Save(10*vg.Inch, 4*vg.Inch, outputFile)
	kcore.Expect(err, "failed to save plot")
}
