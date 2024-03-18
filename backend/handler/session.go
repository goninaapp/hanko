package handler

import (
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
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
	hub := sentryecho.GetHubFromContext(c)

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
		if hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.AddBreadcrumb(&sentry.Breadcrumb{
					Category: "auth",
					Message:  "failed to find refresh token",
					Level:    sentry.LevelError,
					Data: map[string]interface{}{
						"headers": c.Request().Header,
						"cookies": c.Request().Cookies(),
					},
					Timestamp: time.Now(),
				}, 5)

				hub.CaptureMessage("missing refresh token")
			})
		}

		return echo.NewHTTPError(http.StatusUnauthorized, "missing refresh token")
	}

	err := handler.manager.ExchangeRefreshToken(token, c)
	if err != nil {
		if hub != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.AddBreadcrumb(&sentry.Breadcrumb{
					Category: "auth",
					Message:  "failed exchange refresh token",
					Level:    sentry.LevelError,
					Data: map[string]interface{}{
						"headers": c.Request().Header,
						"cookies": c.Request().Cookies(),
						"error":   err,
					},
					Timestamp: time.Now(),
				}, 5)

				hub.CaptureException(err)
			})
		}

		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}

	return c.NoContent(http.StatusOK)
}
