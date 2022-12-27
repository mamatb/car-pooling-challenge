package main

import (
	"sync"
)

// car struct to use internally
type car struct {
	id             int
	seatsTotal     int
	seatsAvailable int
}

// car struct to parse JSON API calls
type carJSONAPI struct {
	Id    int `json:"id" binding:"required"`
	Seats int `json:"seats" binding:"required"`
}

// cars struct to use concurrently
type carsStruct struct {
	sync.Mutex
	m     map[int]car
	pools []map[int]car // cars are organized in 6 pools (maps) according to available seats
}

// group struct to use internally
type group struct {
	id      int
	arrival int
	people  int
	idCar   int
}

// group struct to parse form API calls
type groupFORMAPI struct {
	Id int `form:"ID" binding:"required"`
}

// group struct to parse JSON API calls
type groupJSONAPI struct {
	Id     int `json:"id" binding:"required"`
	People int `json:"people" binding:"required"`
}

// groups struct to use concurrently
type groupsStruct struct {
	sync.Mutex
	groupsArrived int
	m             map[int]group
	queues        [][]group // groups are organized in 6 queues (slices) according to number of people
}

// cars getter
func (cars *carsStruct) Get(idCar int) (int, int, int) {
	cars.Lock()
	defer cars.Unlock()
	if c, ok := cars.m[idCar]; ok {
		return c.id, c.seatsTotal, c.seatsAvailable
	}
	return 0, 0, 0
}

// reset resets the internal "database" with cars
func (cars *carsStruct) reset() {
	cars.Lock()
	defer cars.Unlock()
	cars.m = make(map[int]car)
	cars.pools = make([]map[int]car, 6)
	for i := range cars.pools {
		cars.pools[i] = make(map[int]car)
	}
}

// reset resets the internal "database" with groups
func (groups *groupsStruct) reset() {
	groups.Lock()
	defer groups.Unlock()
	groups.groupsArrived = 0
	groups.m = make(map[int]group)
	groups.queues = make([][]group, 6)
	for i := range groups.queues {
		groups.queues[i] = make([]group, 0)
	}
}

// loadCars loads available cars into the internal "database"
func (cars *carsStruct) loadCars(carsToLoad []carJSONAPI) {
	cars.Lock()
	defer cars.Unlock()
	for _, cJSONAPI := range carsToLoad {
		cars.m[cJSONAPI.Id] = car{
			id:             cJSONAPI.Id,
			seatsTotal:     cJSONAPI.Seats,
			seatsAvailable: cJSONAPI.Seats,
		}
		cars.pools[cJSONAPI.Seats-1][cJSONAPI.Id] = cars.m[cJSONAPI.Id]
	}
}

// rideTryGroup gets a journey for a group or enqueues it for later
func (cars *carsStruct) rideTryGroup(groupToRide groupJSONAPI, groups *groupsStruct) int {
	cars.Lock()
	groups.Lock()
	defer cars.Unlock()
	defer groups.Unlock()
	g := group{
		id:     groupToRide.Id,
		people: groupToRide.People,
	}
	if _, ok := groups.m[g.id]; !ok { // check for already existing groups sending journey requests
		groups.groupsArrived++
		g.arrival = groups.groupsArrived
		groups.m[g.id] = g
		for i := g.people - 1; i < len(cars.pools); i++ { // any car with enough available seats will do
			if len(cars.pools[i]) > 0 {
				for _, c := range cars.pools[i] {
					cars.ride(c, g, groups)
					return 0
				}
			}
		}
		groups.queues[g.people-1] = append(groups.queues[g.people-1], g)
		return 1
	}
	return -1
}

// ride gets a journey for a specific car+group, must be called during cars.Lock() and groups.Lock()
func (cars *carsStruct) ride(c car, g group, groups *groupsStruct) {
	groups.m[g.id] = group{
		id:      g.id,
		arrival: g.arrival,
		people:  g.people,
		idCar:   c.id,
	}
	delete(cars.pools[c.seatsAvailable-1], c.id)
	cars.m[c.id] = car{
		id:             c.id,
		seatsTotal:     c.seatsTotal,
		seatsAvailable: c.seatsAvailable - g.people,
	}
	if cars.m[c.id].seatsAvailable > 0 { // cars without available seats are not stored in the pools
		cars.pools[cars.m[c.id].seatsAvailable-1][c.id] = cars.m[c.id]
	}
}

// dropGroup tries to drop off a group (whether traveling or not) and to reuse the car (if any)
func (groups *groupsStruct) dropGroup(groupToDrop groupFORMAPI, cars *carsStruct) int {
	cars.Lock()
	groups.Lock()
	defer cars.Unlock()
	defer groups.Unlock()
	if g, ok := groups.m[groupToDrop.Id]; ok {
		delete(groups.m, g.id)
		if g.idCar > 0 {
			c := cars.m[g.idCar]
			if c.seatsAvailable > 0 { // cars without available seats are not stored in the pools
				delete(cars.pools[c.seatsAvailable-1], c.id)
			}
			cars.m[c.id] = car{
				id:             c.id,
				seatsTotal:     c.seatsTotal,
				seatsAvailable: c.seatsAvailable + g.people,
			}
			cars.pools[cars.m[c.id].seatsAvailable-1][c.id] = cars.m[c.id]
			groups.rideTryCar(cars.m[c.id], cars) // reuse new available seats
			return 0
		}
		return 1 // groups in queues not updated, check for residual groups when using them
	}
	return -1
}

// rideTryCar gets a journey for a car or stores it, must be called during cars.Lock() and groups.Lock()
func (groups *groupsStruct) rideTryCar(c car, cars *carsStruct) {
	for i := range groups.queues { // check for residual groups at the beginning of each queue
		var j int
		for j = 0; j < len(groups.queues[i]); j++ {
			if g, ok := groups.m[groups.queues[i][j].id]; ok && groups.queues[i][j] == g {
				break
			}
		}
		groups.queues[i] = groups.queues[i][j:]
	}
	var g group
	gQueue := -1
	for i := c.seatsAvailable - 1; i >= 0; i-- {
		if len(groups.queues[i]) > 0 {
			if g.id == 0 || groups.queues[i][0].arrival < g.arrival { // arrival order should be kept when possible
				g = groups.queues[i][0]
				gQueue = i
			}
		}
	}
	if gQueue >= 0 {
		groups.queues[gQueue] = groups.queues[gQueue][1:]
		cars.ride(c, g, groups)
		groups.rideTryCar(cars.m[c.id], cars) // repeat in case there are still available seats
	}
}

// locateGroup locates the car a group is traveling with, if any
func (groups *groupsStruct) locateGroup(groupToLocate groupFORMAPI) int {
	groups.Lock()
	defer groups.Unlock()
	if g, ok := groups.m[groupToLocate.Id]; ok {
		return g.idCar
	}
	return -1
}
