package main

import (
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/olekukonko/tablewriter"
	"github.com/tkrajina/gpxgo/gpx"
)

func Score(points []gpx.GPXPoint) float64 {
	distance := Distance(points)
	if distance == 0 {
		return 0
	}
	// TODO: Expect NullableFloat64
	slope := (points[len(points)-1].Elevation.Value() - points[0].Elevation.Value()) / distance * 100.0

	return math.Abs(slope) * slope * distance / 1000.0
}

func Category(score float64) string {
	switch {
	case score < 35:
		return "NO"
	case score < 80:
		return "Cat 4"
	case score < 180:
		return "Cat 3"
	case score < 250:
		return "Cat 2"
	case score < 600:
		return "Cat 1"
	default:
		return "HC"
	}
}

type Climb struct {
	start int
	end   int
}

func (c Climb) Score(points []gpx.GPXPoint) float64 {
	return Score(points[c.start:c.end])
}

func BestUpHill(points []gpx.GPXPoint, start int, end int) Climb {
	bestScore := Score(points[start:end])
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(points[i:end])
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := bestStart; i < end; i++ {
		score := Score(points[bestStart:i])
		if score > bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	return Climb{bestStart, bestEnd}
}

func BestDownHill(points []gpx.GPXPoint, start int, end int) Climb {
	bestScore := Score(points[start:end])
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(points[i:end])
		if score < bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := bestStart; i < end; i++ {
		score := Score(points[bestStart:i])
		if score < bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	return Climb{bestStart, bestEnd}
}

func Distance(points []gpx.GPXPoint) float64 {
	distance := 0.0
	for i, point := range points {
		if i > 0 {
			distance += point.Distance2D(&points[i-1])
		}
	}
	return distance
}

func FindAllClimbs(points []gpx.GPXPoint, start int, end int) []Climb {
	from := Distance(points[:start])
	to := from + Distance(points[start:end])
	fmt.Printf("Searching climbs between %.1fkm and %.1fkm (%d-%d)\n", from/1000, to/1000, start, end)
	climb := BestUpHill(points, start, end)
	descent := BestDownHill(points, start, end)
	climbs := []Climb{}

	if climb.Score(points) < 35 {
		if descent.Score(points) < -35 {
			if descent.start > start {
				climbs = append(climbs, FindAllClimbs(points, start, descent.start)...)
			}
			if end > descent.end {
				climbs = append(climbs, FindAllClimbs(points, descent.end, end)...)
			}
		}
	} else {
		climbs = append(climbs, climb)
		if climb.start > start {
			climbs = append(climbs, FindAllClimbs(points, start, climb.start)...)
		}
		if end > climb.end {
			climbs = append(climbs, FindAllClimbs(points, climb.end, end)...)
		}
	}

	return climbs
}

func main() {
	example, err := gpx.ParseFile("examples/2022-07-21.Pogacar.gpx")
	if err != nil {
		panic(err)
	}
	segment := example.Tracks[0].Segments[0]
	climbs := FindAllClimbs(segment.Points, 0, len(segment.Points)-1)
	slices.SortFunc(climbs, func(i, j Climb) int { return i.start - j.start })
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Score", "Distance", "Category", "Slope", "From", "To"})
	for _, climb := range climbs {
		table.Append([]string{fmt.Sprintf("%d", int(climb.Score(segment.Points))), fmt.Sprintf("%.1fkm", Distance(segment.Points[climb.start:climb.end])/1000), Category(climb.Score(segment.Points)), fmt.Sprintf("%.1f%%", (segment.Points[climb.end].Elevation.Value()-segment.Points[climb.start].Elevation.Value())/Distance(segment.Points[climb.start:climb.end])*100), fmt.Sprintf("%.1fkm", Distance(segment.Points[:climb.start])/1000), fmt.Sprintf("%.1fkm", Distance(segment.Points[:climb.end])/1000)})
	}
	table.Render()
}
