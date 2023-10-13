package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/teamhanko/hanko/backend/persistence"
	"net/http"
)

type HealthHandler struct {
	persister persistence.Persister
}

func NewHealthHandler(persister persistence.Persister) *HealthHandler {
	return &HealthHandler{
		persister: persister,
	}
}

func (handler *HealthHandler) Ready(c echo.Context) error {
	err := handler.persister.Health()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]bool{"ready": false})
	}

	return c.JSON(http.StatusOK, map[string]bool{"ready": true})
}

func (handler *HealthHandler) Alive(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"alive": true})
}
