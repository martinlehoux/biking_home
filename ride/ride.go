package ride

import (
	"errors"
	"io"
	"math"
	"os"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type Point struct {
	DistanceM  float64
	ElevationM float64
	Coord      geodist.Coord
	Timestamp  time.Time
}

type Ride struct {
	points []Point
}

func (r *Ride) check() {
	kcore.Assert(len(r.points) > 0, "no points in ride")
}

type RideParser interface {
	Parse(reader io.Reader) (Ride, error)
}

func ParseFile(parser RideParser, filename string) (Ride, error) {
	f, err := os.Open(filename)
	if err != nil {
		return Ride{}, err
	}
	defer f.Close()
	return parser.Parse(f)
}

type GPXRideParser struct{}

func (p GPXRideParser) Parse(reader io.Reader) (Ride, error) {
	content, err := gpx.Parse(reader)
	if err != nil {
		return Ride{}, err
	}
	segment := content.Tracks[0].Segments[0]
	points := make([]Point, len(segment.Points))
	distance := 0.0
	for i, p := range segment.Points {
		if i != 0 {
			distance += p.Distance2D(&segment.Points[i-1])
		}
		if i != 0 && distance == 0 {
			return Ride{}, errors.New("zero distance")
		}
		if p.Elevation.Null() {
			return Ride{}, errors.New("points without elevation")
		}
		points[i] = Point{DistanceM: distance, ElevationM: p.Elevation.Value(), Coord: geodist.Coord{Lat: p.Latitude, Lon: p.Longitude}, Timestamp: p.Timestamp}
	}
	ride := Ride{points}
	ride.check()
	return ride, nil
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

func (r *Ride) DifficultyScore() float64 {
	points := r.points
	if len(points) < 2 {
		return 0
	}
	last := points[len(points)-1]
	if last.DistanceM <= 0 {
		return 0
	}
	score := 0.0
	i := 0
	for start := 0.0; start < last.DistanceM; start += 100 {
		end := math.Min(start+100, last.DistanceM)
		for i < len(points)-1 && points[i+1].DistanceM <= start {
			i++
		}
		startElevation := interpolateElevation(points, i, start)
		for i < len(points)-1 && points[i+1].DistanceM < end {
			i++
		}
		endElevation := interpolateElevation(points, i, end)
		slope := (endElevation - startElevation) / (end - start)
		if slope > 0 {
			score += (end - start) / 1000 * (slope * 100) * (slope * 100)
		}
	}
	return score
}

func interpolateElevation(points []Point, i int, distance float64) float64 {
	if i+1 >= len(points) {
		return points[i].ElevationM
	}
	start := points[i]
	end := points[i+1]
	t := (distance - start.DistanceM) / (end.DistanceM - start.DistanceM)
	return start.ElevationM + t*(end.ElevationM-start.ElevationM)
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

func PlotScore(r *Ride, startKm, endKm float64, outputFile string) {
	r.check()

	startIndex := 0
	for i, p := range r.points {
		if p.DistanceM >= startKm*1000 {
			startIndex = i
			break
		}
	}

	pts1 := make(plotter.XYs, 0)
	endIndex := 0
	for i := startIndex + 1; i < len(r.points); i++ {
		if r.points[i].DistanceM > endKm*1000 {
			endIndex = i
			break
		}
		score1 := Score(r.points, startIndex, i)
		pts1 = append(pts1, plotter.XY{
			X: r.points[i].DistanceM / 1000, // Convert distance to kilometers
			Y: score1,
		})
	}
	pts2 := make(plotter.XYs, 0)
	for i := startIndex; i < endIndex; i++ {
		score2 := Score(r.points, i, endIndex)
		pts2 = append(pts2, plotter.XY{
			X: r.points[i].DistanceM / 1000, // Convert distance to kilometers
			Y: score2,
		})
	}

	p := plot.New()
	p.Title.Text = "Ride Score Profile"
	p.X.Label.Text = "Distance (km)"
	p.Y.Label.Text = "Score"

	line1, err := plotter.NewLine(pts1)
	kcore.Expect(err, "failed to create line plot for score1")
	line1.Color = plotutil.Color(0) // First line color
	maxIndex := 0
	maxValue := pts1[0].Y
	for i, pt := range pts1 {
		if pt.Y > maxValue {
			maxValue = pt.Y
			maxIndex = i
		}
	}
	println("Max value of line 1 is at distance:", pts1[maxIndex].X, "km")

	line2, err := plotter.NewLine(pts2)
	kcore.Expect(err, "failed to create line plot for score2")
	line2.Color = plotutil.Color(1) // Second line color

	p.Add(line1, line2)
	p.Legend.Add("Score (start to i)", line1)
	p.Legend.Add("Score (i to end)", line2)

	err = p.Save(10*vg.Inch, 4*vg.Inch, outputFile)
	kcore.Expect(err, "failed to save plot")
}
