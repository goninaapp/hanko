package handler

import (
	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/teamhanko/hanko/backend/config"
	"github.com/teamhanko/hanko/backend/session"
	"net/http"
	"strings"
	"time"
)

type SessionHandler struct {
	enableHeader bool
	cookieName   string
	manager      session.Manager
}

func NewSessionHandler(cfg *config.Config, manager session.Manager) *SessionHandler {
	return &SessionHandler{
		enableHeader: cfg.Session.EnableAuthTokenHeader,
		cookieName:   cfg.Session.Cookie.Name + "-refresh",
		manager:      manager,
	}
}

func (handler *SessionHandler) ExchangeRefreshToken(c echo.Context) error {
	token := ""

	header := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		token = strings.TrimPrefix(header, "Bearer ")
	} else {
		cookie, _ := c.Cookie(handler.cookieName)
		if cookie != nil {
			token = cookie.Value
		}
	}

	if token == "" {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "auth",
			Message:  "failed to find refresh token",
			Level:    sentry.LevelError,
			Data: map[string]interface{}{
				"headers": c.Request().Header,
				"cookies": c.Request().Cookies(),
			},
			Timestamp: time.Now(),
		})

		sentry.CaptureMessage("missing refresh token")

		return echo.NewHTTPError(http.StatusUnauthorized, "missing refresh token")
	}

	err := handler.manager.ExchangeRefreshToken(token, c)
	if err != nil {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "auth",
			Message:  "failed to exchange refresh token",
			Level:    sentry.LevelError,
			Data: map[string]interface{}{
				"headers": c.Request().Header,
				"cookies": c.Request().Cookies(),
				"error":   err,
			},
			Timestamp: time.Now(),
		})

		sentry.CaptureException(err)

		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}

	return c.NoContent(http.StatusOK)
}
