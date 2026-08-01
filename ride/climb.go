package ride

import (
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/martinlehoux/kagamigo/kcore"
)

const ClimbDistanceMinimum = 500

type Climb struct {
	rideStart int
	rideEnd   int
	points    []Point
}

func (climb Climb) Duration() time.Duration {
	return climb.points[len(climb.points)-1].Timestamp.Sub(climb.points[0].Timestamp)
}

func (climb Climb) Speed() float64 {
	return (climb.End().DistanceM - climb.Start().DistanceM) / climb.Duration().Seconds()
}

func (climb Climb) Start() Point {
	return climb.points[0]
}

func (climb Climb) End() Point {
	return climb.points[len(climb.points)-1]
}

func (climb Climb) String() string {
	start := climb.points[0]
	end := climb.points[len(climb.points)-1]
	score := Score(climb.points, 0, len(climb.points)-1)
	return fmt.Sprintf("%.1fkm-%.1fkm: %.1fkm at %.1f%% (%d pts - %s)", start.DistanceM/1000, end.DistanceM/1000, (end.DistanceM-start.DistanceM)/1000, Slope(start, end)*100, int(score), Category(score))
}

func (climb Climb) Score() float64 {
	return Score(climb.points, 0, len(climb.points)-1)
}

func (climb Climb) DifficultyScore() float64 {
	return difficultyScore(climb.points)
}

func Slope(start, end Point) float64 {
	return (end.ElevationM - start.ElevationM) / (end.DistanceM - start.DistanceM)
}

func Score(points []Point, start int, end int) float64 {
	kcore.Assert(end > start, "no points for score")
	distance := points[end].DistanceM - points[start].DistanceM
	if distance == 0 {
		return 0
	}
	dElevation := points[end].ElevationM - points[start].ElevationM

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
	kcore.Assert(bestStart < bestEnd, "empty climb")
	climb := Climb{rideStart: bestStart, rideEnd: bestEnd, points: points[bestStart : bestEnd+1]}

	return climb
}

func climbsBetween(points []Point, start int, end int) []Climb {
	climbs := []Climb{}
	if points[end].DistanceM-points[start].DistanceM < ClimbDistanceMinimum {
		return climbs
	}
	slog.Debug("Searching climbs between", slog.Int("start", int(points[start].DistanceM)), slog.Int("end", int(points[end].DistanceM)))
	highest := start
	for i := start; i <= end; i++ {
		if points[i].ElevationM > points[highest].ElevationM {
			highest = i
		}
	}
	// TODO: Use descent to reduce recursion
	if points[highest].DistanceM-points[start].DistanceM < ClimbDistanceMinimum {
		return climbsBetween(points, start+1, end)
	}
	climb := bestClimbBetween(points, start, highest)
	if climb.Score() >= 35 && climb.End().DistanceM-climb.Start().DistanceM >= ClimbDistanceMinimum {
		slog.Debug("Found climb between", slog.Int("start", int(climb.Start().DistanceM)), slog.Int("end", int(climb.End().DistanceM)))
		climbs = append(climbs, climb)
	}
	climbs = append(climbs, climbsBetween(points, start, climb.rideStart)...)
	climbs = append(climbs, climbsBetween(points, climb.rideEnd, end)...)

	return climbs
}

func (ride *Ride) AllClimbs() []Climb {
	climbs := climbsBetween(ride.points, 0, len(ride.points)-1)
	slices.SortFunc(climbs, climbCmpStart)
	return climbs
}

func climbCmpStart(a, b Climb) int { return a.rideStart - b.rideStart }
