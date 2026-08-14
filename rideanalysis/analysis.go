package rideanalysis

import (
	"math"

	"github.com/jftuga/geodist"
	"github.com/martinlehoux/biking_home/mountain_pass"
	"github.com/martinlehoux/biking_home/official_climb"
	"github.com/martinlehoux/biking_home/ride"
)

type Result struct {
	Climbs    []AnalyzedClimb
	Crossings []mountain_pass.Crossing
}

type AnalyzedClimb struct {
	Segment         ride.Climb
	Name            string
	Score           float64
	Category        string
	DistanceM       float64
	SlopePercent    float64
	Cotacol         float64
	OfficialClimbID int64
	OfficialName    string
}

func Analyze(parsed ride.Ride, passes []mountain_pass.MountainPass, officialClimbs []official_climb.OfficialClimb, matchPolicy official_climb.MatchPolicy) Result {
	result := Result{
		Climbs:    make([]AnalyzedClimb, 0),
		Crossings: make([]mountain_pass.Crossing, 0),
	}
	for _, detectedClimb := range parsed.AllClimbs() {
		displayClimb := detectedClimb
		if matchedPass, ok := mountain_pass.MatchClimb(displayClimb, passes, 300, 50); ok {
			displayClimb.Name = matchedPass.Name
		}
		matchedOfficial, officialFound := official_climb.MatchClimb(detectedClimb, officialClimbs, matchPolicy)
		if officialFound {
			startIndex := nearestCoordIndex(parsed, matchedOfficial.StartCoord)
			endIndex := nearestCoordIndex(parsed, matchedOfficial.EndCoord)
			if startIndex < endIndex {
				displayClimb = parsed.ClimbFromIndexes(startIndex, endIndex)
				displayClimb.Name = matchedOfficial.Name
			} else {
				officialFound = false
				matchedOfficial = official_climb.OfficialClimb{}
			}
		}
		score := displayClimb.Score()
		result.Climbs = append(result.Climbs, AnalyzedClimb{
			Segment:         displayClimb,
			Name:            displayClimb.Name,
			Score:           score,
			Category:        ride.Category(score),
			DistanceM:       displayClimb.EndDistanceM() - displayClimb.StartDistanceM(),
			SlopePercent:    ride.Slope(parsed, displayClimb.StartIndex(), displayClimb.EndIndex()) * 100,
			Cotacol:         displayClimb.DifficultyScore(),
			OfficialClimbID: officialClimbID(matchedOfficial, officialFound),
			OfficialName:    officialClimbName(matchedOfficial, officialFound),
		})
	}
	for _, crossing := range mountain_pass.DetectCrossings(parsed, passes, 100, 25) {
		result.Crossings = append(result.Crossings, crossing)
	}
	return result
}

func nearestCoordIndex(parsed ride.Ride, target geodist.Coord) int {
	bestIndex := 0
	bestDistance := math.Inf(1)
	for index := 0; index < parsed.Len(); index++ {
		_, distanceKm := geodist.HaversineDistance(parsed.Coord(index), target)
		if distanceKm < bestDistance {
			bestDistance = distanceKm
			bestIndex = index
		}
	}
	return bestIndex
}

func officialClimbID(climb official_climb.OfficialClimb, found bool) int64 {
	if !found {
		return 0
	}
	return climb.ID
}

func officialClimbName(climb official_climb.OfficialClimb, found bool) string {
	if !found {
		return ""
	}
	return climb.Name
}
