package ride_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/stretchr/testify/assert"
	"github.com/tkrajina/gpxgo/gpx"
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
	points := []ride.Point{ride.NewPoint(0, 0)}
	distance := 0.0
	elevation := 0.0
	for _, section := range b.sections {
		curSecDist := 0.0
		for curSecDist < section.distance {
			curSecDist += b.precision
			elevation += b.precision * section.slope
			points = append(points, ride.NewPoint(
				distance+curSecDist,
				elevation,
			))
		}
		distance += curSecDist
	}
	return ride.FromPoints(points)
}

func TestClimbThenFalseFlat(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("2km at 7%").WithSection("10km at 1%").Build()
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 1)
	assert.Equal(t, "0.0km-2.0km: 2.0km at 7.0% (98 pts - Cat 3)", r.String(climbs[0]))
}

func TestSmallClimbWithDescentInsideFalseFlat(t *testing.T) {
	r := RideBuilder{precision: 100}.WithSection("10km at 1%").WithSection("2km at 7%").WithSection("1km at -7%").WithSection("10km at 1%").Build()
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 1)
	assert.Equal(t, "10.0km-12.0km: 2.0km at 7.0% (98 pts - Cat 3)", r.String(climbs[0]))
}

func TestPogacar20220721(t *testing.T) {
	gpxContent, err := gpx.ParseFile("../examples/2022-07-21.Pogacar.gpx")
	assert.NoError(t, err)
	r := ride.FromGPX(gpxContent)
	climbs := r.AllClimbs()

	assert.Len(t, climbs, 6)
	assert.Equal(t, "0.6km-5.6km: 5.0km at 3.5% (62 pts - Cat 4)", r.String(climbs[0]))
	assert.Equal(t, "42.9km-45.2km: 2.3km at 5.0% (59 pts - Cat 4)", r.String(climbs[1]))
	assert.Equal(t, "60.0km-76.4km: 16.4km at 7.2% (854 pts - HC)", r.String(climbs[2]))
	assert.Equal(t, "83.8km-85.9km: 2.0km at 5.4% (59 pts - Cat 4)", r.String(climbs[3]))
	assert.Equal(t, "99.2km-109.3km: 10.2km at 8.4% (710 pts - HC)", r.String(climbs[4]))
	assert.Equal(t, "128.6km-142.2km: 13.6km at 7.8% (832 pts - HC)", r.String(climbs[5]))
}
