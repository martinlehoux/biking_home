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

type Ride struct {
	distances  []float64
	elevations []float64
	coords     []geodist.Coord
	timestamps []time.Time
}

func (r *Ride) check() {
	kcore.Assert(r.Len() > 0, "no points in ride")
	kcore.Assert(len(r.distances) == len(r.elevations), "ride columns have different lengths")
	kcore.Assert(len(r.distances) == len(r.coords), "ride columns have different lengths")
	kcore.Assert(len(r.distances) == len(r.timestamps), "ride columns have different lengths")
}

func (r Ride) Len() int {
	return len(r.distances)
}

func (r Ride) DistanceM(i int) float64 { return r.distances[i] }

func (r Ride) ElevationM(i int) float64 { return r.elevations[i] }

func (r Ride) Coord(i int) geodist.Coord { return r.coords[i] }

func (r Ride) Timestamp(i int) time.Time { return r.timestamps[i] }

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
	if len(content.Tracks) == 0 || len(content.Tracks[0].Segments) == 0 {
		return Ride{}, errors.New("ride has no track segment")
	}
	segment := content.Tracks[0].Segments[0]
	if len(segment.Points) == 0 {
		return Ride{}, errors.New("ride has no track points")
	}
	distances := make([]float64, 0, len(segment.Points))
	elevations := make([]float64, 0, len(segment.Points))
	coords := make([]geodist.Coord, 0, len(segment.Points))
	timestamps := make([]time.Time, 0, len(segment.Points))
	distance := 0.0
	previous := segment.Points[0]
	if previous.Elevation.Null() {
		return Ride{}, errors.New("points without elevation")
	}
	distances = append(distances, 0)
	elevations = append(elevations, previous.Elevation.Value())
	coords = append(coords, geodist.Coord{Lat: previous.Latitude, Lon: previous.Longitude})
	timestamps = append(timestamps, previous.Timestamp)
	for i := 1; i < len(segment.Points); i++ {
		p := segment.Points[i]
		distance += p.Distance2D(&previous)
		previous = p
		if distance == 0 {
			continue
		}
		if p.Elevation.Null() {
			return Ride{}, errors.New("points without elevation")
		}
		distances = append(distances, distance)
		elevations = append(elevations, p.Elevation.Value())
		coords = append(coords, geodist.Coord{Lat: p.Latitude, Lon: p.Longitude})
		timestamps = append(timestamps, p.Timestamp)
	}
	if len(distances) < 2 {
		return Ride{}, errors.New("zero distance")
	}
	ride := Ride{distances: distances, elevations: elevations, coords: coords, timestamps: timestamps}
	ride.check()
	return ride, nil
}

func FromColumns(distances []float64, elevations []float64, coords []geodist.Coord, timestamps []time.Time) Ride {
	ride := Ride{distances: distances, elevations: elevations, coords: coords, timestamps: timestamps}
	ride.check()
	return ride
}

func (r *Ride) ScoreFromKm(start, end float64) float64 {
	i := 0
	j := 0
	for k, distance := range r.distances {
		if i == 0 && distance >= start*1000 {
			i = k
		}
		if j == 0 && distance >= end*1000 {
			j = k
			break
		}
	}
	return Score(*r, i, j)
}

func (r *Ride) DifficultyScore() float64 {
	return difficultyScore(*r, 0, r.Len()-1)
}

func difficultyScore(r Ride, startIndex, endIndex int) float64 {
	if endIndex-startIndex < 1 {
		return 0
	}
	startDistance := r.DistanceM(startIndex)
	lastDistance := r.DistanceM(endIndex)
	if lastDistance <= startDistance {
		return 0
	}
	score := 0.0
	i := startIndex
	for start := startDistance; start < lastDistance; start += 100 {
		end := math.Min(start+100, lastDistance)
		for i < endIndex && r.DistanceM(i+1) <= start {
			i++
		}
		startElevation := interpolateElevation(r, i, start)
		for i < endIndex && r.DistanceM(i+1) < end {
			i++
		}
		endElevation := interpolateElevation(r, i, end)
		slope := (endElevation - startElevation) / (end - start)
		if slope > 0 {
			score += (end - start) / 1000 * (slope * 100) * (slope * 100)
		}
	}
	return score
}

func interpolateElevation(r Ride, i int, distance float64) float64 {
	if i+1 >= r.Len() {
		return r.ElevationM(i)
	}
	startDistance := r.DistanceM(i)
	endDistance := r.DistanceM(i + 1)
	t := (distance - startDistance) / (endDistance - startDistance)
	return r.ElevationM(i) + t*(r.ElevationM(i+1)-r.ElevationM(i))
}

func (r *Ride) ClimbFromDist(startDist, endDist float64) Climb {
	start, end := 0, 0
	for i, distance := range r.distances {
		if start == 0 && distance >= startDist {
			start = i
		}
		if end == 0 && distance >= endDist {
			end = i
			break
		}
	}
	return Climb{ride: *r, rideStart: start, rideEnd: end}
}

func Plot(r *Ride, outputFile string) {
	r.check()

	pts := make(plotter.XYs, r.Len())
	for i := 0; i < r.Len(); i++ {
		pts[i].X = r.DistanceM(i) / 1000 // Convert distance to kilometers
		pts[i].Y = r.ElevationM(i)
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
	for i := 0; i < r.Len(); i++ {
		if r.DistanceM(i) >= startKm*1000 {
			startIndex = i
			break
		}
	}

	pts1 := make(plotter.XYs, 0)
	endIndex := 0
	for i := startIndex + 1; i < r.Len(); i++ {
		if r.DistanceM(i) > endKm*1000 {
			endIndex = i
			break
		}
		score1 := Score(*r, startIndex, i)
		pts1 = append(pts1, plotter.XY{
			X: r.DistanceM(i) / 1000, // Convert distance to kilometers
			Y: score1,
		})
	}
	pts2 := make(plotter.XYs, 0)
	for i := startIndex; i < endIndex; i++ {
		score2 := Score(*r, i, endIndex)
		pts2 = append(pts2, plotter.XY{
			X: r.DistanceM(i) / 1000, // Convert distance to kilometers
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
