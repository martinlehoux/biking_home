package strava

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
	"github.com/tkrajina/gpxgo/gpx"
)

type Activity struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	SportType string    `json:"sport_type"`
	StartDate time.Time `json:"start_date"`
}

func (c *Client) ListActivities(after time.Time, types ...string) ([]Activity, error) {
	allowed := map[string]bool{}
	for _, t := range types {
		allowed[t] = true
	}
	var all []Activity
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("per_page", "200")
		q.Set("page", strconv.Itoa(page))
		q.Set("after", strconv.FormatInt(after.Unix(), 10))
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

func (c *Client) ActivityStreams(id int64) (latlng [][2]float64, altitude []float64, seconds []float64, err error) {
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

func WriteActivityGPX(file string, activity Activity, latlng [][2]float64, altitude, seconds []float64) error {
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
		return kcore.Wrap(err, "failed to build GPX XML")
	}
	return os.WriteFile(file, xmlData, 0o644)
}
