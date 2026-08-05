package web

import (
	"fmt"
	"time"

	"github.com/martinlehoux/biking_home/rides"
)

type RideView struct {
	rides.Ride
	Cotacol         string
	CotacolPer100Km string
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
