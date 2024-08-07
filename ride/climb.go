package ride

import (
	"fmt"
	"math"
	"slices"

	"github.com/martinlehoux/kagamigo/kcore"
)

const ClimbDistanceMinimum = 500

type Climb struct {
	start int
	end   int
}

type Point struct {
	distance  float64
	elevation float64
}

func NewPoint(distance float64, elevation float64) Point {
	return Point{
		distance:  distance,
		elevation: elevation,
	}
}

func Slope(start, end Point) float64 {
	return (end.elevation - start.elevation) / (end.distance - start.distance)
}

func Score(points []Point, start int, end int) float64 {
	kcore.Assert(end > start, "no points for score")
	distance := points[end].distance - points[start].distance
	if distance == 0 {
		return 0
	}
	dElevation := points[end].elevation - points[start].elevation

	return math.Abs(dElevation) * dElevation / distance * 100.0 * 100.0 / 1000.0
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

func bestClimbBetween(points []Point, start int, end int) Climb {
	kcore.Assert(end > start, "empty points")

	bestScore := Score(points, start, end)
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(points, i, end)
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := end; i > bestStart; i-- {
		score := Score(points, bestStart, i)
		if score > bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	for i := bestStart; i < bestEnd; i++ {
		score := Score(points, i, bestEnd)
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	climb := Climb{bestStart, bestEnd}

	kcore.Assert(climb.start < climb.end, "empty climb")
	return climb
}

func climbsBetween(points []Point, start int, end int) []Climb {
	climbs := []Climb{}
	if points[end].distance-points[start].distance < ClimbDistanceMinimum {
		return climbs
	}
	fmt.Printf("Searching climbs between %.1fkm and %.1fkm\n", points[start].distance/1000, points[end].distance/1000)
	highest := start
	for i := start; i <= end; i++ {
		if points[i].elevation > points[highest].elevation {
			highest = i
		}
	}
	// TODO: Use descent to reduce recursion
	if points[highest].distance-points[start].distance < ClimbDistanceMinimum {
		return climbsBetween(points, start+1, end)
	}
	climb := bestClimbBetween(points, start, highest)
	if Score(points, climb.start, climb.end) >= 35 && points[climb.end].distance-points[climb.start].distance >= ClimbDistanceMinimum {
		fmt.Printf("Found climb between %.1fkm and %.1fkm\n", points[climb.start].distance/1000, points[climb.end].distance/1000)
		climbs = append(climbs, climb)
	}
	climbs = append(climbs, climbsBetween(points, start, climb.start)...)
	climbs = append(climbs, climbsBetween(points, climb.end, end)...)

	return climbs
}

func (ride *Ride) AllClimbs() []Climb {
	climbs := climbsBetween(ride.points, 0, len(ride.points)-1)
	slices.SortFunc(climbs, climbCmpStart)
	return climbs
}

func climbCmpStart(a, b Climb) int { return a.start - b.start }
