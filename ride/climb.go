package ride

import (
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/kagamigo/kcore"
)

const ClimbDistanceMinimum = 500

type Climb struct {
	ride      Ride
	rideStart int
	rideEnd   int
	Name      string
}

func (climb Climb) Duration() time.Duration {
	return climb.ride.Timestamp(climb.rideEnd).Sub(climb.ride.Timestamp(climb.rideStart))
}

func (climb Climb) Speed() float64 {
	return (climb.EndDistanceM() - climb.StartDistanceM()) / climb.Duration().Seconds()
}

func (climb Climb) StartIndex() int { return climb.rideStart }

func (climb Climb) EndIndex() int { return climb.rideEnd }

func (climb Climb) StartDistanceM() float64 {
	return climb.ride.DistanceM(climb.rideStart)
}

func (climb Climb) EndDistanceM() float64 {
	return climb.ride.DistanceM(climb.rideEnd)
}

// TopIndex returns the highest point inside the climb, used as the crest anchor.
func (climb Climb) TopIndex() int {
	top := climb.rideStart
	for i := climb.rideStart; i <= climb.rideEnd; i++ {
		if climb.ride.ElevationM(i) > climb.ride.ElevationM(top) {
			top = i
		}
	}
	return top
}

func (climb Climb) TopDistanceM() float64 {
	return climb.ride.DistanceM(climb.TopIndex())
}

func (climb Climb) TopElevationM() float64 {
	return climb.ride.ElevationM(climb.TopIndex())
}

func (climb Climb) TopCoord() geodist.Coord {
	return climb.ride.Coord(climb.TopIndex())
}

func (climb Climb) String() string {
	startDistance := climb.StartDistanceM()
	endDistance := climb.EndDistanceM()
	score := climb.Score()
	body := fmt.Sprintf("%.1fkm-%.1fkm: %.1fkm at %.1f%% (%d pts - %s)", startDistance/1000, endDistance/1000, (endDistance-startDistance)/1000, Slope(climb.ride, climb.rideStart, climb.rideEnd)*100, int(score), Category(score))
	if climb.Name == "" {
		return body
	}
	return climb.Name + ": " + body
}

func (climb Climb) Score() float64 {
	return Score(climb.ride, climb.rideStart, climb.rideEnd)
}

func (climb Climb) DifficultyScore() float64 {
	return difficultyScore(climb.ride, climb.rideStart, climb.rideEnd)
}

func Slope(r Ride, start, end int) float64 {
	return (r.ElevationM(end) - r.ElevationM(start)) / (r.DistanceM(end) - r.DistanceM(start))
}

func Score(r Ride, start, end int) float64 {
	kcore.Assert(end > start, "no points for score")
	distance := r.DistanceM(end) - r.DistanceM(start)
	if distance == 0 {
		return 0
	}
	dElevation := r.ElevationM(end) - r.ElevationM(start)

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

func bestClimbBetween(r Ride, start, end int) Climb {
	kcore.Assert(end > start, "empty points")

	bestScore := Score(r, start, end)
	bestStart := start
	for i := start; i < end; i++ {
		score := Score(r, i, end)
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	bestEnd := end
	for i := end; i > bestStart; i-- {
		score := Score(r, bestStart, i)
		if score > bestScore {
			bestEnd = i
			bestScore = score
		}
	}
	for i := bestStart; i < bestEnd; i++ {
		score := Score(r, i, bestEnd)
		if score > bestScore {
			bestStart = i
			bestScore = score
		}
	}
	kcore.Assert(bestStart < bestEnd, "empty climb")
	return Climb{ride: r, rideStart: bestStart, rideEnd: bestEnd}
}

func climbsBetween(r Ride, start, end int) []Climb {
	climbs := []Climb{}
	if r.DistanceM(end)-r.DistanceM(start) < ClimbDistanceMinimum {
		return climbs
	}
	slog.Debug("Searching climbs between", slog.Int("start", int(r.DistanceM(start))), slog.Int("end", int(r.DistanceM(end))))
	highest := start
	for i := start; i <= end; i++ {
		if r.ElevationM(i) > r.ElevationM(highest) {
			highest = i
		}
	}
	// TODO: Use descent to reduce recursion
	if r.DistanceM(highest)-r.DistanceM(start) < ClimbDistanceMinimum {
		return climbsBetween(r, start+1, end)
	}
	climb := bestClimbBetween(r, start, highest)
	if climb.Score() >= 35 && climb.EndDistanceM()-climb.StartDistanceM() >= ClimbDistanceMinimum {
		slog.Debug("Found climb between", slog.Int("start", int(climb.StartDistanceM())), slog.Int("end", int(climb.EndDistanceM())))
		climbs = append(climbs, climb)
	}
	climbs = append(climbs, climbsBetween(r, start, climb.rideStart)...)
	climbs = append(climbs, climbsBetween(r, climb.rideEnd, end)...)

	return climbs
}

func (ride *Ride) AllClimbs() []Climb {
	climbs := climbsBetween(*ride, 0, ride.Len()-1)
	slices.SortFunc(climbs, climbCmpStart)
	return climbs
}

func climbCmpStart(a, b Climb) int { return a.rideStart - b.rideStart }
