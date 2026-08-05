package strava

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
)

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
	latlng, altitude, seconds, err := c.activityStreams(id)
	if err != nil {
		return Activity{}, nil, err
	}
	gpxData, err := activityGPX(activity, latlng, altitude, seconds)
	if err != nil {
		return Activity{}, nil, err
	}
	return activity, gpxData, nil
}

func (c *Client) activityStreams(id int64) (latlng [][2]float64, altitude []float64, seconds []float64, err error) {
	path := fmt.Sprintf("/activities/%d/streams?keys=latlng,altitude,time&key_by_type=true", id)
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
	}
	err = c.GetJSON(path, &streams)
	if err != nil {
		return nil, nil, nil, kcore.Wrap(err, "failed to fetch Strava streams")
	}
	if streams.LatLng != nil {
		latlng = streams.LatLng.Data
	}
	if streams.Altitude != nil {
		altitude = streams.Altitude.Data
	}
	if streams.Time != nil {
		seconds = streams.Time.Data
	}
	return latlng, altitude, seconds, nil
}

func activityGPX(activity Activity, latlng [][2]float64, altitude, seconds []float64) ([]byte, error) {
	points := make([]gpx.GPXPoint, len(latlng))
	for i := range latlng {
		point := gpx.GPXPoint{}
		point.Latitude = latlng[i][0]
		point.Longitude = latlng[i][1]
		if i < len(altitude) {
			point.Elevation = *gpx.NewNullableFloat64(altitude[i])
		}
		if i < len(seconds) {
			point.Timestamp = activity.StartDate.Add(time.Duration(seconds[i]) * time.Second)
		}
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
	xmlData, err := doc.ToXml(gpx.ToXmlParams{})
	if err != nil {
		return nil, kcore.Wrap(err, "failed to build GPX XML")
	}
	return xmlData, nil
}
