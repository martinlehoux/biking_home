package ride

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
)

const garminTrackPointExtensionNamespace = "http://www.garmin.com/xmlschemas/TrackPointExtension/v1"

type Ride struct {
	distances  []float64
	elevations []float64
	coords     []geodist.Coord
	timestamps []time.Time
	heartRates []float64
	cadences   []float64
	powers     []float64
}

func (r *Ride) check() {
	kcore.Assert(r.Len() > 0, "no points in ride")
	kcore.Assert(len(r.distances) == len(r.elevations), "ride columns have different lengths")
	kcore.Assert(len(r.distances) == len(r.coords), "ride columns have different lengths")
	kcore.Assert(len(r.distances) == len(r.timestamps), "ride columns have different lengths")
	for _, column := range [][]float64{r.heartRates, r.cadences, r.powers} {
		if len(column) > 0 {
			kcore.Assert(len(r.distances) == len(column), "ride columns have different lengths")
		}
	}
}

func (r Ride) Len() int {
	return len(r.distances)
}

func (r Ride) DistanceM(i int) float64 { return r.distances[i] }

func (r Ride) ElevationM(i int) float64 { return r.elevations[i] }

func (r Ride) Coord(i int) geodist.Coord { return r.coords[i] }

func (r Ride) Timestamp(i int) time.Time { return r.timestamps[i] }

func (r Ride) HeartRateBpm(i int) (float64, bool) {
	return optionalMetric(r.heartRates, i)
}

func (r Ride) CadenceRpm(i int) (float64, bool) {
	return optionalMetric(r.cadences, i)
}

func (r Ride) PowerW(i int) (float64, bool) {
	return optionalMetric(r.powers, i)
}

func optionalMetric(column []float64, i int) (float64, bool) {
	if len(column) == 0 {
		return 0, false
	}
	return column[i], true
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
	heartRates := make([]float64, 0, len(segment.Points))
	cadences := make([]float64, 0, len(segment.Points))
	powers := make([]float64, 0, len(segment.Points))
	heartRateComplete := true
	cadenceComplete := true
	powerComplete := true
	distance := 0.0
	previous := segment.Points[0]
	if previous.Elevation.Null() {
		return Ride{}, errors.New("points without elevation")
	}
	distances = append(distances, 0)
	elevations = append(elevations, previous.Elevation.Value())
	coords = append(coords, geodist.Coord{Lat: previous.Latitude, Lon: previous.Longitude})
	timestamps = append(timestamps, previous.Timestamp)
	sample, err := metricSampleForPoint(previous)
	if err != nil {
		return Ride{}, err
	}
	appendMetric(&heartRates, &heartRateComplete, sample.heartRate)
	appendMetric(&cadences, &cadenceComplete, sample.cadence)
	appendMetric(&powers, &powerComplete, sample.power)
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
		sample, err = metricSampleForPoint(p)
		if err != nil {
			return Ride{}, err
		}
		appendMetric(&heartRates, &heartRateComplete, sample.heartRate)
		appendMetric(&cadences, &cadenceComplete, sample.cadence)
		appendMetric(&powers, &powerComplete, sample.power)
	}
	if len(distances) < 2 {
		return Ride{}, errors.New("zero distance")
	}
	var heartRatesColumn, cadencesColumn, powersColumn []float64
	if heartRateComplete {
		heartRatesColumn = heartRates
	}
	if cadenceComplete {
		cadencesColumn = cadences
	}
	if powerComplete {
		powersColumn = powers
	}
	return fromColumns(distances, elevations, coords, timestamps, heartRatesColumn, cadencesColumn, powersColumn), nil
}

func FromColumns(distances []float64, elevations []float64, coords []geodist.Coord, timestamps []time.Time) Ride {
	return fromColumns(distances, elevations, coords, timestamps, nil, nil, nil)
}

func fromColumns(distances []float64, elevations []float64, coords []geodist.Coord, timestamps []time.Time, heartRates, cadences, powers []float64) Ride {
	ride := Ride{
		distances:  distances,
		elevations: elevations,
		coords:     coords,
		timestamps: timestamps,
		heartRates: heartRates,
		cadences:   cadences,
		powers:     powers,
	}
	ride.check()
	return ride
}

type metricSample struct {
	heartRate *float64
	cadence   *float64
	power     *float64
}

func metricSampleForPoint(point gpx.GPXPoint) (metricSample, error) {
	heartRate, err := metricValue(point, "hr")
	if err != nil {
		return metricSample{}, err
	}
	cadence, err := metricValue(point, "cad")
	if err != nil {
		return metricSample{}, err
	}
	power, err := metricValue(point, "watts")
	if err != nil {
		return metricSample{}, err
	}
	return metricSample{heartRate: heartRate, cadence: cadence, power: power}, nil
}

func appendMetric(column *[]float64, complete *bool, value *float64) {
	if value == nil {
		*complete = false
		*column = append(*column, 0)
		return
	}
	*column = append(*column, *value)
}

func metricValue(point gpx.GPXPoint, name string) (*float64, error) {
	trackPointExtension, found := point.Extensions.GetNode(gpx.NamespaceURL(garminTrackPointExtensionNamespace), "TrackPointExtension")
	if !found {
		return nil, nil
	}
	node, found := trackPointExtension.GetNode(name)
	if !found {
		return nil, nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(node.Data), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Garmin %s value %q: %w", name, node.Data, err)
	}
	return &value, nil
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

const CotacolAlgorithmVersion = "v1"

func Cotacol(ride Ride) float64 {
	return difficultyScore(ride, 0, ride.Len()-1)
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

func (r *Ride) ClimbFromIndexes(startIndex, endIndex int) Climb {
	kcore.Assert(startIndex >= 0 && endIndex < r.Len() && startIndex < endIndex, "invalid climb indexes")
	return Climb{ride: *r, rideStart: startIndex, rideEnd: endIndex}
}
