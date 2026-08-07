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
const parserTestGPXMetricsPrefix = `<?xml version="1.0"?><gpx xmlns="http://www.topografix.com/GPX/1/1" xmlns:gpxtpx="http://www.garmin.com/xmlschemas/TrackPointExtension/v1" version="1.1"><trk><trkseg>`

func TestGPXRideParserSkipsStationaryPoints(t *testing.T) {
	data := parserTestGPXPrefix +
		`<trkpt lat="43.0" lon="5.0"><ele>100</ele></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>101</ele></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>102</ele></trkpt>` +
		`<trkpt lat="43.001" lon="5.001"><ele>103</ele></trkpt>` +
		parserTestGPXSuffix

	parsed, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 2, parsed.Len())
	assert.Greater(t, parsed.DistanceM(1), 0.0)
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

func TestGPXRideParserReadsGarminMetrics(t *testing.T) {
	data := parserTestGPXMetricsPrefix +
		`<trkpt lat="43.0" lon="5.0"><ele>100</ele><extensions><gpxtpx:TrackPointExtension><gpxtpx:hr>90</gpxtpx:hr><gpxtpx:cad>70</gpxtpx:cad><gpxtpx:watts>200</gpxtpx:watts></gpxtpx:TrackPointExtension></extensions></trkpt>` +
		`<trkpt lat="43.0" lon="5.0"><ele>101</ele><extensions><gpxtpx:TrackPointExtension><gpxtpx:hr>91</gpxtpx:hr><gpxtpx:cad>71</gpxtpx:cad><gpxtpx:watts>201</gpxtpx:watts></gpxtpx:TrackPointExtension></extensions></trkpt>` +
		`<trkpt lat="43.001" lon="5.001"><ele>102</ele><extensions><gpxtpx:TrackPointExtension><gpxtpx:hr>92</gpxtpx:hr><gpxtpx:cad>72</gpxtpx:cad><gpxtpx:watts>202</gpxtpx:watts></gpxtpx:TrackPointExtension></extensions></trkpt>` +
		parserTestGPXSuffix

	parsed, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 2, parsed.Len())

	heartRate, found := parsed.HeartRateBpm(1)
	require.True(t, found)
	assert.Equal(t, 92.0, heartRate)
	cadence, found := parsed.CadenceRpm(1)
	require.True(t, found)
	assert.Equal(t, 72.0, cadence)
	power, found := parsed.PowerW(1)
	require.True(t, found)
	assert.Equal(t, 202.0, power)
}

func TestGPXRideParserDropsIncompleteMetricColumn(t *testing.T) {
	data := parserTestGPXMetricsPrefix +
		`<trkpt lat="43.0" lon="5.0"><ele>100</ele><extensions><gpxtpx:TrackPointExtension><gpxtpx:hr>90</gpxtpx:hr><gpxtpx:cad>70</gpxtpx:cad><gpxtpx:watts>200</gpxtpx:watts></gpxtpx:TrackPointExtension></extensions></trkpt>` +
		`<trkpt lat="43.001" lon="5.001"><ele>102</ele><extensions><gpxtpx:TrackPointExtension><gpxtpx:hr>92</gpxtpx:hr><gpxtpx:watts>0</gpxtpx:watts></gpxtpx:TrackPointExtension></extensions></trkpt>` +
		parserTestGPXSuffix

	parsed, err := (ride.GPXRideParser{}).Parse(strings.NewReader(data))
	require.NoError(t, err)

	_, found := parsed.CadenceRpm(0)
	assert.False(t, found)
	heartRate, found := parsed.HeartRateBpm(1)
	require.True(t, found)
	assert.Equal(t, 92.0, heartRate)
	power, found := parsed.PowerW(1)
	require.True(t, found)
	assert.Equal(t, 0.0, power)
}
