package main

import (
	"log"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

var (
	cars   carsStruct
	groups groupsStruct
)

// init gets executed as soon as the package is imported
func init() {
	resetState()
}

// resetState resets the internal "database" with cars and groups
func resetState() {
	cars.reset()
	groups.reset()
}

// getStatus indicates the service is ready
func getStatus(c *gin.Context) {
	if c.Request.Method != "GET" {
		c.Status(http.StatusBadRequest)
		log.Println("Error getStatus: request format failure.")
		return
	}
	log.Println("Success getStatus.")
}

// putCars loads available cars and resets the service state
func putCars(c *gin.Context) {
	if c.Request.Method != "PUT" {
		c.Status(http.StatusBadRequest)
		log.Println("Error putCars: request format failure.")
		return
	}
	if ok, _ := regexp.MatchString("^application/json(;.*)?$", c.GetHeader("Content-Type")); !ok {
		c.Status(http.StatusBadRequest)
		log.Println("Error putCars: expected headers failure.")
		return
	}
	var carsToLoad []carJSONAPI
	if err := c.ShouldBindJSON(&carsToLoad); err != nil {
		c.Status(http.StatusBadRequest)
		log.Println("Error putCars: payload can't be unmarshalled.", err)
		return
	}
	resetState()
	cars.loadCars(carsToLoad)
	log.Println("Success putCars: cars loaded.")
}

// postJourney gets a journey for a group or enqueues it for later
func postJourney(c *gin.Context) {
	if c.Request.Method != "POST" {
		c.Status(http.StatusBadRequest)
		log.Println("Error postJourney: request format failure.")
		return
	}
	if ok, _ := regexp.MatchString("^application/json(;.*)?$", c.GetHeader("Content-Type")); !ok {
		c.Status(http.StatusBadRequest)
		log.Println("Error postJourney: expected headers failure.")
		return
	}
	var groupToRide groupJSONAPI
	if err := c.ShouldBindJSON(&groupToRide); err != nil {
		c.Status(http.StatusBadRequest)
		log.Println("Error postJourney: payload can't be unmarshalled.", err)
		return
	}
	switch cars.rideTryGroup(groupToRide, &groups) {
	case -1:
		c.Status(http.StatusAccepted)
		log.Println("Info postJourney: group already enqueued or traveling.")
		return
	case 1:
		c.Status(http.StatusAccepted)
		log.Println("Success postJourney: group enqueued for traveling.")
		return
	}
	log.Println("Success postJourney: group traveling.")
}

// postDropoff tries to drop off a group traveling (or not)
func postDropoff(c *gin.Context) {
	if c.Request.Method != "POST" {
		c.Status(http.StatusBadRequest)
		log.Println("Error postDropoff: request format failure.")
		return
	}
	if ok, _ := regexp.MatchString("^application/x-www-form-urlencoded(;.*)?$", c.GetHeader("Content-Type")); !ok {
		c.Status(http.StatusBadRequest)
		log.Println("Error postDropoff: expected headers failure.")
		return
	}
	var groupToDrop groupFORMAPI
	if err := c.ShouldBind(&groupToDrop); err != nil {
		c.Status(http.StatusBadRequest)
		log.Println("Error postDropoff: payload can't be unmarshalled.", err)
		return
	}
	switch groups.dropGroup(groupToDrop, &cars) {
	case -1:
		c.Status(http.StatusNotFound)
		log.Println("Info postDropoff: group not found.")
		return
	case 1:
		c.Status(http.StatusNoContent)
		log.Println("Success postDropoff: waiting group dropped off.")
		return
	}
	log.Println("Success postDropoff: traveling group dropped off.")
}

// postLocate locates the car a group is traveling with, if any
func postLocate(c *gin.Context) {
	if c.Request.Method != "POST" {
		c.Status(http.StatusBadRequest)
		log.Println("Error postLocate: request format failure.")
		return
	}
	if ok, _ := regexp.MatchString("^application/x-www-form-urlencoded(;.*)?$", c.GetHeader("Content-Type")); !ok {
		c.Status(http.StatusBadRequest)
		log.Println("Error postLocate: expected headers failure.")
		return
	}
	var groupToLocate groupFORMAPI
	if err := c.ShouldBind(&groupToLocate); err != nil {
		c.Status(http.StatusBadRequest)
		log.Println("Error postLocate: payload can't be unmarshalled.", err)
		return
	}
	idCar := groups.locateGroup(groupToLocate)
	switch idCar {
	case -1:
		c.Status(http.StatusNotFound)
		log.Println("Info postLocate: group not found.")
		return
	case 0:
		c.Status(http.StatusNoContent)
		log.Println("Success postLocate: waiting group located.")
		return
	}
	var carToLocate carJSONAPI
	carToLocate.Id, carToLocate.Seats, _ = cars.Get(idCar)
	c.JSON(http.StatusOK, carToLocate)
	log.Println("Success postLocate: traveling group located.")
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Any("/status", getStatus)
	router.Any("/cars", putCars)
	router.Any("/journey", postJourney)
	router.Any("/dropoff", postDropoff)
	router.Any("/locate", postLocate)
	router.Run(":9091")
}
