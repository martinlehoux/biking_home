package ride

import "github.com/jftuga/geodist"

type RideClimbsIndex interface {
	Insert(ride Ride)
	Similar(ride Ride, climb Climb) []Climb
}

type FlatRideClimbsIndex struct {
	sensibility float64
	climbs      []Climb
}

func NewFlatRideClimbsIndex(sensibility float64) *FlatRideClimbsIndex {
	return &FlatRideClimbsIndex{
		climbs:      make([]Climb, 0),
		sensibility: sensibility,
	}
}

func (index *FlatRideClimbsIndex) Insert(ride Ride) {
	index.climbs = append(index.climbs, ride.AllClimbs()...)
}

func (index *FlatRideClimbsIndex) Similar(ride Ride, climb Climb) []Climb {
	results := make([]Climb, 0)
	for _, c := range index.climbs {
		_, startDistance := geodist.HaversineDistance(c.Start().Coord, climb.Start().Coord)
		_, endDistance := geodist.HaversineDistance(c.End().Coord, climb.End().Coord)
		if startDistance <= index.sensibility/1000 && endDistance <= index.sensibility/1000 {
			results = append(results, c)
		}
	}
	return results
}
