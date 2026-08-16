package web

import (
	"fmt"
	"net/url"
	"time"

	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/official_climb"
	"github.com/martinlehoux/biking_home/ride"
	"github.com/martinlehoux/biking_home/rideanalysis"
	"github.com/martinlehoux/biking_home/rides"
)

type RideView struct {
	rides.Ride
	Cotacol         string
	CotacolPer100Km string
}

type RideDetailView struct {
	RideView
	Route       GeoJSONFeatureCollection
	HasRoute    bool
	Profile     RideProfile
	RouteError  string
	ActionError string
	Notice      string
}

type RideProfile struct {
	Points    []RideProfilePoint    `json:"points"`
	Climbs    []RideProfileClimb    `json:"climbs"`
	Crossings []RideProfileCrossing `json:"crossings"`
}

func (profile RideProfile) OfficialClimbs() []RideProfileClimb {
	climbs := make([]RideProfileClimb, 0)
	for _, climb := range profile.Climbs {
		if climb.OfficialClimbID > 0 {
			climbs = append(climbs, climb)
		}
	}
	return climbs
}

func (profile RideProfile) UnmatchedClimbs() []RideProfileClimb {
	climbs := make([]RideProfileClimb, 0)
	for _, climb := range profile.Climbs {
		if climb.OfficialClimbID == 0 {
			climbs = append(climbs, climb)
		}
	}
	return climbs
}

type RideProfilePoint struct {
	DistanceKm float64 `json:"distanceKm"`
	ElevationM float64 `json:"elevationM"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

type RideProfileClimb struct {
	Index           int     `json:"-"`
	StartKm         float64 `json:"startKm"`
	EndKm           float64 `json:"endKm"`
	TopKm           float64 `json:"topKm"`
	TopElevationM   float64 `json:"topElevationM"`
	Name            string  `json:"name"`
	Category        string  `json:"category"`
	DistanceKm      float64 `json:"distanceKm"`
	SlopePercent    float64 `json:"slopePercent"`
	Cotacol         float64 `json:"cotacol"`
	OfficialClimbID int64   `json:"officialClimbId,omitempty"`
	OfficialName    string  `json:"officialName,omitempty"`
	StartIndex      int     `json:"startIndex"`
	EndIndex        int     `json:"endIndex"`
}

type RideProfileCrossing struct {
	DistanceKm    float64 `json:"distanceKm"`
	PassElevation float64 `json:"passElevationM"`
	RideElevation float64 `json:"rideElevationM"`
	DistanceToM   float64 `json:"distanceToM"`
	ElevationDiff float64 `json:"elevationDiffM"`
	Name          string  `json:"name"`
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

func officialClimbCreateURL(id int64) string {
	return fmt.Sprintf("/rides/%d/official-climbs", id)
}

func buildRideDetailView(item rides.Ride, parsed ride.Ride, passes []mountain_pass.MountainPass, officialClimbs []official_climb.OfficialClimb, matchPolicy official_climb.MatchPolicy) RideDetailView {
	coordinates := make([][]float64, parsed.Len())
	for i := 0; i < parsed.Len(); i++ {
		coordinate := parsed.Coord(i)
		coordinates[i] = []float64{coordinate.Lon, coordinate.Lat}
	}
	profile := buildRideProfile(parsed, passes, officialClimbs, matchPolicy)
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
		Profile:  profile,
	}
}

func buildRideProfile(parsed ride.Ride, passes []mountain_pass.MountainPass, officialClimbs []official_climb.OfficialClimb, matchPolicy official_climb.MatchPolicy) RideProfile {
	profile := RideProfile{
		Points:    make([]RideProfilePoint, parsed.Len()),
		Climbs:    make([]RideProfileClimb, 0),
		Crossings: make([]RideProfileCrossing, 0),
	}
	for i := 0; i < parsed.Len(); i++ {
		coordinate := parsed.Coord(i)
		profile.Points[i] = RideProfilePoint{
			DistanceKm: parsed.DistanceM(i) / 1000,
			ElevationM: parsed.ElevationM(i),
			Latitude:   coordinate.Lat,
			Longitude:  coordinate.Lon,
		}
	}
	analysis := rideanalysis.Analyze(parsed, passes, officialClimbs, matchPolicy)
	for _, analyzedClimb := range analysis.Climbs {
		segment := analyzedClimb.Segment
		climb := RideProfileClimb{
			Index:           len(profile.Climbs),
			StartKm:         segment.StartDistanceM() / 1000,
			EndKm:           segment.EndDistanceM() / 1000,
			TopKm:           segment.TopDistanceM() / 1000,
			TopElevationM:   segment.TopElevationM(),
			Name:            analyzedClimb.Name,
			Category:        analyzedClimb.Category,
			DistanceKm:      analyzedClimb.DistanceM / 1000,
			SlopePercent:    analyzedClimb.SlopePercent,
			Cotacol:         analyzedClimb.Cotacol,
			OfficialClimbID: analyzedClimb.OfficialClimbID,
			OfficialName:    analyzedClimb.OfficialName,
			StartIndex:      segment.StartIndex(),
			EndIndex:        segment.EndIndex(),
		}
		profile.Climbs = append(profile.Climbs, climb)
	}
	for _, crossing := range analysis.Crossings {
		profile.Crossings = append(profile.Crossings, RideProfileCrossing{
			DistanceKm:    crossing.RideDistanceM / 1000,
			PassElevation: float64(crossing.Pass.Elevation),
			RideElevation: crossing.RideElevation,
			DistanceToM:   crossing.DistanceToM,
			ElevationDiff: crossing.ElevationDiff,
			Name:          crossing.Pass.Name,
		})
	}
	return profile
}

type SyncPageData struct {
	From    string
	To      string
	Error   string
	Notice  string
	HasAuth bool
}

type SyncProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Imported  int `json:"imported"`
	Skipped   int `json:"skipped"`
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

func formatCotacol(cotacol float64) string {
	return fmt.Sprintf("%.1f", cotacol)
}

func formatCotacolPer100Km(cotacol, distanceM float64) string {
	distanceKm := distanceM / 1000
	if distanceKm <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", cotacol*100/distanceKm)
}

func formatRouteIndex(index int) string {
	return fmt.Sprintf("%d", index)
}

func formatClimbNumber(index int) string {
	return fmt.Sprintf("%d", index+1)
}

func formatClimbCount(count int) string {
	return fmt.Sprintf("%d", count)
}

func formatProfileDistance(distanceKm float64) string {
	return fmt.Sprintf("%.1f km", distanceKm)
}

func formatSlope(slopePercent float64) string {
	return fmt.Sprintf("%.1f%%", slopePercent)
}

func officialProfileLabel(name string) string {
	return "Elevation profile for " + name
}

func rideDetailColumnsClass(unmatchedClimbCount int) string {
	if unmatchedClimbCount == 0 {
		return "ride-detail-columns ride-detail-columns-single"
	}
	return "ride-detail-columns"
}
