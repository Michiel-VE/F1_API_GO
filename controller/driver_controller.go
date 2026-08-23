package controller

import (
	"fmt"
	"net/http"

	"f1-api-go/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DriverController struct {
	driverService            *services.DriverService
	driverRaceResultsService *services.DriverRaceResultsService
}

func NewDriverController(driverService *services.DriverService, driverRaceResultsService *services.DriverRaceResultsService) *DriverController {
	return &DriverController{
		driverService:            driverService,
		driverRaceResultsService: driverRaceResultsService,
	}
}

func (d *DriverController) GetDriverById(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid driver ID")
		return
	}

	driver, err := d.driverService.GetDriverById(c.Request.Context(), id)
	if err != nil {
		SendError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, driver)
}

func (d *DriverController) GetDriverByLastName(c *gin.Context) {
	lastName := c.Param("lastName")
	driver, err := d.driverService.GetDriversByLastName(c.Request.Context(), lastName)
	if err != nil {
		SendError(c, http.StatusNotFound, "Driver not found")
		return
	}

	c.JSON(http.StatusOK, driver)
}

func (d *DriverController) GetDriverCareerHistory(c *gin.Context) {
	name := c.Param("name")
	history, err := d.driverService.GetDriverCareerHistory(c.Request.Context(), name)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	SendListResponse(c, history)
}

func (d *DriverController) GetDriverRaceResults(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid driver UUID format")
		return
	}

	season := c.Query("season")
	results, err := d.driverRaceResultsService.GetDriverRaceResults(c.Request.Context(), id, season)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, results)
}

func (d *DriverController) GetDriversBySeason(c *gin.Context) {
	season := c.Param("season")
	drivers, err := d.driverService.GetDriversBySeason(c.Request.Context(), season)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	SendListResponse(c, drivers)
}

func (d *DriverController) GetDriverByPermanentNumber(c *gin.Context) {
	permNumStr := c.Query("permanentNumber")
	if permNumStr == "" {
		SendError(c, http.StatusBadRequest, "Permanent number is required")
		return
	}

	var permNum int
	_, err := fmt.Sscanf(permNumStr, "%d", &permNum)
	if err != nil {
		SendError(c, http.StatusBadRequest, "Invalid permanent number format")
		return
	}

	drivers, err := d.driverService.GetDriversByPermanentNumber(c.Request.Context(), permNum)
	if err != nil || len(drivers) == 0 {
		SendError(c, http.StatusNotFound, "Driver not found")
		return
	}

	c.JSON(http.StatusOK, drivers[0])
}

func (d *DriverController) GetAllDrivers(c *gin.Context) {
	drivers, err := d.driverService.GetAllDrivers(c.Request.Context())
	if err != nil {
		SendError(c, http.StatusInternalServerError, err.Error())
		return
	}

	SendListResponse(c, drivers)
}
