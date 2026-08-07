package ride_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/stretchr/testify/assert"
)

type RideBuilderSection struct {
	slope    float64
	distance float64
}

type RideBuilder struct {
	precision float64
	sections  []RideBuilderSection
}

// Example: 13.4km at 7.5%
func (b RideBuilder) WithSection(input string) RideBuilder {
	inputs := strings.Split(input, " ")
	kcore.Assert(strings.HasSuffix(inputs[0], "km"), "wrong unit")
	distanceKm, err := strconv.ParseFloat(strings.TrimSuffix(inputs[0], "km"), 64)
	kcore.Expect(err, "faile to parse kms")
	kcore.Assert(strings.HasSuffix(inputs[2], "%"), "wrong unit")
	slopePercent, err := strconv.ParseFloat(strings.TrimSuffix(inputs[2], "%"), 64)
	kcore.Expect(err, "failed to parse %")

	b.sections = append(b.sections, RideBuilderSection{
		slope:    slopePercent / 100,
		distance: distanceKm * 1000,
	})

	return b
}

func (b RideBuilder) Build() ride.Ride {
	distances := []float64{0}
	elevations := []float64{0}
	coords := []geodist.Coord{{}}
	timestamps := []time.Time{{}}
	distance := 0.0
	elevation := 0.0
	for _, section := range b.sections {
		curSecDist := 0.0
		for curSecDist < section.distance {
			curSecDist += b.precision
			elevation += b.precision * section.slope
			distances = append(distances, distance+curSecDist)
			elevations = append(elevations, elevation)
			coords = append(coords, geodist.Coord{})
			timestamps = append(timestamps, timestamps[len(timestamps)-1].Add(time.Second))
		}
		distance += curSecDist
	}
	return ride.FromColumns(distances, elevations, coords, timestamps)
}

var parser = ride.GPXRideParser{}

func TestClimbThenFalseFlat(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("2km at 7%").WithSection("10km at 1%").Build()
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 1)
	assert.Equal(t, "0.0km-2.0km: 2.0km at 7.0% (98 pts - Cat 3)", climbs[0].String())
}

func TestSmallClimbWithDescentInsideFalseFlat(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("10km at 1%").WithSection("2km at 7%").WithSection("1km at -7%").WithSection("10km at 1%").Build()
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 1)
	assert.Equal(t, "10.0km-12.0km: 2.0km at 7.0% (98 pts - Cat 3)", climbs[0].String())
}

func TestPogacar20220721(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2022-07-21.Pogacar.gpx")
	assert.NoError(t, err)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 6)
	assert.Equal(t, "0.6km-5.6km: 5.0km at 3.5% (62 pts - Cat 4)", climbs[0].String())
	assert.Equal(t, "42.9km-45.2km: 2.3km at 5.0% (59 pts - Cat 4)", climbs[1].String())
	assert.Equal(t, "60.0km-76.4km: 16.4km at 7.2% (854 pts - HC)", climbs[2].String())
	assert.Equal(t, "83.8km-85.9km: 2.0km at 5.4% (59 pts - Cat 4)", climbs[3].String())
	assert.Equal(t, "99.2km-109.3km: 10.2km at 8.4% (710 pts - HC)", climbs[4].String())
	assert.Equal(t, "128.6km-142.2km: 13.6km at 7.8% (832 pts - HC)", climbs[5].String())
}

func TestBouclesVerdon2024(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2024-06-29.BouclesVerdon.gpx")
	assert.NoError(t, err)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 6)
	assert.Equal(t, "3.6km-13.8km: 10.3km at 2.1% (44 pts - Cat 4)", climbs[0].String()) // TODO: Not steep enough
	assert.Equal(t, "34.7km-37.3km: 2.5km at 5.0% (62 pts - Cat 4)", climbs[1].String())
	assert.Equal(t, "41.9km-51.0km: 9.0km at 2.5% (55 pts - Cat 4)", climbs[2].String()) // TODO: Not steep enough
	assert.Equal(t, "57.6km-60.8km: 3.1km at 6.3% (124 pts - Cat 3)", climbs[3].String())
	assert.Equal(t, "71.7km-74.7km: 3.0km at 5.3% (84 pts - Cat 3)", climbs[4].String())
	assert.Equal(t, "77.4km-78.7km: 1.3km at 5.3% (37 pts - Cat 4)", climbs[5].String())
}

func TestMimetArbois20241229(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2024-12-29.MimetArbois.gpx")
	assert.NoError(t, err)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 2)
	assert.Equal(t, "3.9km-5.3km: 1.4km at 10.3% (153 pts - Cat 3)", climbs[0].String())
	assert.Equal(t, "14.4km-19.8km: 5.4km at 4.3% (98 pts - Cat 3)", climbs[1].String())
}

func TestMimetArbois20250104(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2025-01-04.MimetArbois.gpx")
	assert.NoError(t, err)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 2)
	assert.Equal(t, "4.5km-5.9km: 1.4km at 10.1% (148 pts - Cat 3)", climbs[0].String())
	assert.Equal(t, "12.0km-20.2km: 8.2km at 2.9% (67 pts - Cat 4)", climbs[1].String()) // TODO: Not steep enough
}

// https://www.strava.com/activities/9282265844
func TestAlpesVerdonTour20230617(t *testing.T) {
	r, err := ride.ParseFile(parser, "../examples/2023-06-17.AlpesVerdonTour.gpx")
	assert.NoError(t, err)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 6)
	assert.Equal(t, "0.7km-18.6km: 17.9km at 3.5% (217 pts - Cat 2)", climbs[0].String()) // TODO: Should split in 2 (10km at 4.5% is Cat 2)
	assert.Equal(t, "36.1km-41.6km: 5.4km at 5.8% (182 pts - Cat 2)", climbs[1].String())
	assert.Equal(t, "47.0km-48.2km: 1.2km at 6.1% (45 pts - Cat 4)", climbs[2].String())
	assert.Equal(t, "51.9km-54.7km: 2.8km at 4.0% (45 pts - Cat 4)", climbs[3].String())
	assert.Equal(t, "86.7km-92.8km: 6.1km at 2.7% (44 pts - Cat 4)", climbs[4].String())
	assert.Equal(t, "114.1km-114.7km: 0.6km at 14.3% (118 pts - Cat 3)", climbs[5].String())
}
