package official_climb

import (
	"math"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/ride"
)

func MatchClimb(climb ride.Climb, officialClimbs []OfficialClimb, policy MatchPolicy) (OfficialClimb, bool) {
	if policy.Validate() != nil {
		return OfficialClimb{}, false
	}
	bestDistance := math.Inf(1)
	var best OfficialClimb
	found := false
	for _, official := range officialClimbs {
		startIndex, startDistance := nearestClimbPoint(climb, official.StartCoord)
		endIndex, endDistance := nearestClimbPoint(climb, official.EndCoord)
		if startDistance > policy.EndpointRadiusM || endDistance > policy.EndpointRadiusM {
			continue
		}
		if startIndex >= endIndex {
			continue
		}
		totalDistance := startDistance + endDistance
		if totalDistance < bestDistance || (totalDistance == bestDistance && (!found || official.ID < best.ID)) {
			best = official
			bestDistance = totalDistance
			found = true
		}
	}
	return best, found
}

func nearestClimbPoint(climb ride.Climb, target geodist.Coord) (int, float64) {
	bestIndex := climb.StartIndex()
	bestDistance := math.Inf(1)
	for index := climb.StartIndex(); index <= climb.EndIndex(); index++ {
		distance := distanceM(climb.PointCoord(index), target)
		if distance < bestDistance {
			bestIndex = index
			bestDistance = distance
		}
	}
	return bestIndex, bestDistance
}

func distanceM(a, b geodist.Coord) float64 {
	_, distanceKm := geodist.HaversineDistance(a, b)
	return distanceKm * 1000
}
