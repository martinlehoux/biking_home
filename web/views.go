package web

import (
	"fmt"
	"net/url"
	"time"

	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/rides"
)

type RideView struct {
	rides.Ride
	Cotacol         string
	CotacolPer100Km string
}

type RideDetailView struct {
	RideView
	Route      GeoJSONFeatureCollection
	HasRoute   bool
	RouteError string
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string          `json:"type"`
	Geometry   GeoJSONGeometry `json:"geometry"`
	Properties map[string]any  `json:"properties,omitempty"`
}

type GeoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type RideSort struct {
	Column     string
	Descending bool
}

type RideSortHeader struct {
	Label     string
	Class     string
	URL       string
	AriaSort  string
	Indicator string
}

const (
	rideSortName       = "name"
	rideSortStarted    = "started"
	rideSortDistance   = "distance"
	rideSortMovingTime = "moving_time"
	rideSortElevation  = "elevation"
	rideSortCotacol    = "cotacol"
	rideSortCotacolKm  = "cotacol_100km"
)

var rideSortColumns = map[string]bool{
	rideSortName:       true,
	rideSortStarted:    true,
	rideSortDistance:   true,
	rideSortMovingTime: true,
	rideSortElevation:  true,
	rideSortCotacol:    true,
	rideSortCotacolKm:  true,
}

func parseRideSort(query url.Values) RideSort {
	column := query.Get("sort")
	if !rideSortColumns[column] {
		column = rideSortStarted
	}
	direction := query.Get("dir")
	return RideSort{Column: column, Descending: direction != "asc"}
}

func (s RideSort) databaseColumn() (rides.SortColumn, bool) {
	columns := map[string]rides.SortColumn{
		rideSortName:       rides.SortName,
		rideSortStarted:    rides.SortStartDate,
		rideSortDistance:   rides.SortDistance,
		rideSortMovingTime: rides.SortMovingTime,
		rideSortElevation:  rides.SortElevation,
		rideSortCotacol:    rides.SortCotacol,
		rideSortCotacolKm:  rides.SortCotacolKm,
	}
	column, found := columns[s.Column]
	return column, found
}

func rideSortHeaders(current RideSort) []RideSortHeader {
	columns := []struct {
		key   string
		label string
		class string
	}{
		{key: rideSortName, label: "Ride"},
		{key: rideSortStarted, label: "Started"},
		{key: rideSortDistance, label: "Distance", class: "numeric"},
		{key: rideSortMovingTime, label: "Moving time", class: "numeric"},
		{key: rideSortElevation, label: "Elevation", class: "numeric"},
		{key: rideSortCotacol, label: "Cotacol", class: "numeric"},
		{key: rideSortCotacolKm, label: "Cotacol / 100 km", class: "numeric"},
	}
	headers := make([]RideSortHeader, 0, len(columns))
	for _, column := range columns {
		descending := false
		active := current.Column == column.key
		if active {
			descending = !current.Descending
		}
		query := url.Values{}
		query.Set("sort", column.key)
		if descending {
			query.Set("dir", "desc")
		} else {
			query.Set("dir", "asc")
		}
		headerClass := column.class
		ariaSort := "none"
		indicator := ""
		if active {
			headerClass += " active"
			if current.Descending {
				ariaSort = "descending"
				indicator = "↓"
			} else {
				ariaSort = "ascending"
				indicator = "↑"
			}
		}
		headers = append(headers, RideSortHeader{
			Label:     column.label,
			Class:     headerClass,
			URL:       "/?" + query.Encode(),
			AriaSort:  ariaSort,
			Indicator: indicator,
		})
	}
	return headers
}

func rideDetailURL(id int64) string {
	return fmt.Sprintf("/rides/%d", id)
}

func buildRideDetailView(item rides.Ride, parsed ride.Ride) RideDetailView {
	coordinates := make([][]float64, parsed.Len())
	for i := 0; i < parsed.Len(); i++ {
		coordinate := parsed.Coord(i)
		coordinates[i] = []float64{coordinate.Lon, coordinate.Lat}
	}
	return RideDetailView{
		RideView: buildRideView(item),
		Route: GeoJSONFeatureCollection{
			Type: "FeatureCollection",
			Features: []GeoJSONFeature{{
				Type: "Feature",
				Geometry: GeoJSONGeometry{
					Type:        "LineString",
					Coordinates: coordinates,
				},
			}},
		},
		HasRoute: true,
	}
}

type SyncPageData struct {
	From    string
	To      string
	Error   string
	Notice  string
	HasAuth bool
}

func formatRideDate(value time.Time) string {
	return value.Local().Format("02 Jan 2006, 15:04")
}

func formatDistance(meters float64) string {
	return fmt.Sprintf("%.1f km", meters/1000)
}

func formatDuration(seconds int64) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

func formatElevation(meters float64) string {
	return fmt.Sprintf("%.0f m", meters)
}

func formatCotacol(score float64) string {
	return fmt.Sprintf("%.1f", score)
}

func formatCotacolPer100Km(score, distanceM float64) string {
	distanceKm := distanceM / 1000
	if distanceKm <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", score*100/distanceKm)
}
