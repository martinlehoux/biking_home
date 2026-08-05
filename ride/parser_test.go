package ride_test

import (
	"strings"
	"testing"

	"github.com/martinlehoux/biking_home/ride"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const parserTestGPXPrefix = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" version="1.1"><trk><trkseg>`
const parserTestGPXSuffix = `</trkseg></trk></gpx>`

func TestGPXRideParserSkipsStationaryPoints(t *testing.T) {
	data := parserTestGPXPrefix +
		`<trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>101</ele></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>102</ele></trkpt>` +
		`<trkpt lat="43.001" lon="5.001"><ele>103</ele></trkpt>` +
		parserTestGPXSuffix

	parsed, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.NoError(t, err)
	assert.Len(t, parsed.Points(), 2)
	assert.Greater(t, parsed.Points()[1].DistanceM, 0.0)
}

func TestGPXRideParserRejectsStationaryRide(t *testing.T) {
	data := parserTestGPXPrefix +
		`<trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>101</ele></trkpt>` +
		parserTestGPXSuffix

	_, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.Error(t, err)
	assert.Equal(t, "zero distance", err.Error())
}

func TestGPXRideParserRejectsEmptyTrack(t *testing.T) {
	data := parserTestGPXPrefix + parserTestGPXSuffix

	_, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.Error(t, err)
	assert.Equal(t, "ride has no track points", err.Error())
}
