package strava

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
)

const garminTrackPointExtensionNamespace = "http://www.garmin.com/xmlschemas/TrackPointExtension/v1"

type Activity struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	SportType           string    `json:"sport_type"`
	StartDate           time.Time `json:"start_date"`
	DistanceM           float64   `json:"distance"`
	MovingTimeS         int64     `json:"moving_time"`
	ElapsedTimeS        int64     `json:"elapsed_time"`
	TotalElevationGainM float64   `json:"total_elevation_gain"`
	AverageSpeedMps     float64   `json:"average_speed"`
}

func (c *Client) List(from, to time.Time) ([]Activity, error) {
	return c.list(from, to, "Ride")
}

func (c *Client) list(from, to time.Time, types ...string) ([]Activity, error) {
	allowed := map[string]bool{}
	for _, t := range types {
		allowed[t] = true
	}
	var all []Activity
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("per_page", "200")
		q.Set("page", strconv.Itoa(page))
		if !from.IsZero() {
			q.Set("after", strconv.FormatInt(from.Unix(), 10))
		}
		if !to.IsZero() {
			q.Set("before", strconv.FormatInt(to.Unix(), 10))
		}
		var batch []Activity
		err := c.GetJSON("/athlete/activities?"+q.Encode(), &batch)
		if err != nil {
			return nil, kcore.Wrap(err, "failed to list Strava activities")
		}
		if len(batch) == 0 {
			break
		}
		for _, activity := range batch {
			if len(allowed) == 0 || allowed[activity.Type] || allowed[activity.SportType] {
				all = append(all, activity)
			}
		}
		if len(batch) < 200 {
			break
		}
	}
	return all, nil
}

func (c *Client) Get(id int64) (Activity, []byte, error) {
	var activity Activity
	if err := c.GetJSON(fmt.Sprintf("/activities/%d", id), &activity); err != nil {
		return Activity{}, nil, kcore.Wrap(err, "failed to get Strava activity")
	}
	streams, err := c.activityStreams(id)
	if err != nil {
		return Activity{}, nil, err
	}
	gpxData, err := activityGPX(activity, streams)
	if err != nil {
		return Activity{}, nil, err
	}
	return activity, gpxData, nil
}

type activityStreams struct {
	LatLng    [][2]float64
	Altitude  []float64
	Seconds   []float64
	HeartRate []*float64
	Cadence   []*float64
	Power     []*float64
}

type nullableFloatStream struct {
	Data []*float64 `json:"data"`
}

func (c *Client) activityStreams(id int64) (activityStreams, error) {
	path := fmt.Sprintf("/activities/%d/streams?keys=latlng,altitude,time,heartrate,cadence,watts&key_by_type=true", id)
	var streams struct {
		LatLng *struct {
			Data [][2]float64 `json:"data"`
		} `json:"latlng"`
		Altitude *struct {
			Data []float64 `json:"data"`
		} `json:"altitude"`
		Time *struct {
			Data []float64 `json:"data"`
		} `json:"time"`
		HeartRate *nullableFloatStream `json:"heartrate"`
		Cadence   *nullableFloatStream `json:"cadence"`
		Power     *nullableFloatStream `json:"watts"`
	}
	err := c.GetJSON(path, &streams)
	if err != nil {
		return activityStreams{}, kcore.Wrap(err, "failed to fetch Strava streams")
	}
	result := activityStreams{}
	if streams.LatLng != nil {
		result.LatLng = streams.LatLng.Data
	}
	if streams.Altitude != nil {
		result.Altitude = streams.Altitude.Data
	}
	if streams.Time != nil {
		result.Seconds = streams.Time.Data
	}
	if streams.HeartRate != nil {
		result.HeartRate = streams.HeartRate.Data
	}
	if streams.Cadence != nil {
		result.Cadence = streams.Cadence.Data
	}
	if streams.Power != nil {
		result.Power = streams.Power.Data
	}
	return result, nil
}

func activityGPX(activity Activity, streams activityStreams) ([]byte, error) {
	if len(streams.LatLng) == 0 {
		return nil, errors.New("activity has no track points")
	}
	points := make([]gpx.GPXPoint, len(streams.LatLng))
	for i := range streams.LatLng {
		point := gpx.GPXPoint{}
		point.Latitude = streams.LatLng[i][0]
		point.Longitude = streams.LatLng[i][1]
		if i < len(streams.Altitude) {
			point.Elevation = *gpx.NewNullableFloat64(streams.Altitude[i])
		}
		if i < len(streams.Seconds) {
			point.Timestamp = activity.StartDate.Add(time.Duration(streams.Seconds[i]) * time.Second)
		}
		addTrackPointMetric(&point, "hr", streamValue(streams.HeartRate, i))
		addTrackPointMetric(&point, "cad", streamValue(streams.Cadence, i))
		addTrackPointMetric(&point, "watts", streamValue(streams.Power, i))
		points[i] = point
	}
	doc := gpx.GPX{
		Version: "1.1",
		Creator: "biking_home",
		Name:    activity.Name,
		Tracks: []gpx.GPXTrack{{
			Name:     activity.Name,
			Segments: []gpx.GPXTrackSegment{{Points: points}},
		}},
	}
	doc.RegisterNamespace("gpxtpx", garminTrackPointExtensionNamespace)
	xmlData, err := doc.ToXml(gpx.ToXmlParams{})
	if err != nil {
		return nil, kcore.Wrap(err, "failed to build GPX XML")
	}
	return xmlData, nil
}

func streamValue(stream []*float64, i int) *float64 {
	if i >= len(stream) {
		return nil
	}
	return stream[i]
}

func addTrackPointMetric(point *gpx.GPXPoint, name string, value *float64) {
	if value == nil {
		return
	}
	extension := point.Extensions.GetOrCreateNode(gpx.NamespaceURL(garminTrackPointExtensionNamespace), "TrackPointExtension")
	extension.GetOrCreateNode(name).Data = strconv.FormatFloat(*value, 'f', -1, 64)
}
