package official_climb

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jftuga/geodist"
)

const DefaultMatchRadiusM = 100.0

type OfficialClimb struct {
	ID         int64
	Name       string
	StartCoord geodist.Coord
	EndCoord   geodist.Coord
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MatchPolicy struct {
	EndpointRadiusM float64
}

func DefaultMatchPolicy() MatchPolicy {
	return MatchPolicy{EndpointRadiusM: DefaultMatchRadiusM}
}

func (policy MatchPolicy) Validate() error {
	if math.IsNaN(policy.EndpointRadiusM) || math.IsInf(policy.EndpointRadiusM, 0) || policy.EndpointRadiusM <= 0 {
		return fmt.Errorf("official climb endpoint radius must be greater than zero")
	}
	return nil
}

func (climb OfficialClimb) validate() error {
	if strings.TrimSpace(climb.Name) == "" {
		return fmt.Errorf("official climb name is required")
	}
	if !validCoord(climb.StartCoord) || !validCoord(climb.EndCoord) {
		return fmt.Errorf("official climb coordinates are invalid")
	}
	return nil
}

func validCoord(coord geodist.Coord) bool {
	return coord.Lat >= -90 && coord.Lat <= 90 && coord.Lon >= -180 && coord.Lon <= 180
}
