package ride

import "github.com/jftuga/geodist"

type RideClimbsIndex interface {
	Insert(ride Ride)
	Similar(ride Ride, climb Climb) []Climb
}

type FlatRideClimbsIndex struct {
	sensitivity float64
	climbs      []Climb
}

func NewFlatRideClimbsIndex(sensitivity float64) *FlatRideClimbsIndex {
	return &FlatRideClimbsIndex{
		climbs:      make([]Climb, 0),
		sensitivity: sensitivity,
	}
}

func (index *FlatRideClimbsIndex) Insert(ride Ride) {
	index.climbs = append(index.climbs, ride.AllClimbs()...)
}

func (index *FlatRideClimbsIndex) Similar(ride Ride, climb Climb) []Climb {
	results := make([]Climb, 0)
	for _, c := range index.climbs {
		_, startDistance := geodist.HaversineDistance(c.ride.Coord(c.StartIndex()), climb.ride.Coord(climb.StartIndex()))
		_, endDistance := geodist.HaversineDistance(c.ride.Coord(c.EndIndex()), climb.ride.Coord(climb.EndIndex()))
		if startDistance <= index.sensitivity/1000 && endDistance <= index.sensitivity/1000 {
			results = append(results, c)
		}
	}
	return results
}
