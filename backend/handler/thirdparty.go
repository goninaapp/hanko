package handler

import (
	"fmt"
	"github.com/gobuffalo/pop/v6"
	"github.com/labstack/echo/v4"
	auditlog "github.com/teamhanko/hanko/backend/audit_log"
	"github.com/teamhanko/hanko/backend/config"
	"github.com/teamhanko/hanko/backend/dto"
	"github.com/teamhanko/hanko/backend/persistence"
	"github.com/teamhanko/hanko/backend/persistence/models"
	"github.com/teamhanko/hanko/backend/session"
	"github.com/teamhanko/hanko/backend/thirdparty"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
)

const (
	HankoThirdpartyStateCookie = "hanko_thirdparty_state"
	HankoTokenQuery            = "hanko_token"
)

type ThirdPartyHandler struct {
	auditLogger    auditlog.Logger
	cfg            *config.Config
	persister      persistence.Persister
	sessionManager session.Manager
}

func NewThirdPartyHandler(cfg *config.Config, persister persistence.Persister, sessionManager session.Manager, auditLogger auditlog.Logger) *ThirdPartyHandler {
	return &ThirdPartyHandler{
		auditLogger:    auditLogger,
		cfg:            cfg,
		persister:      persister,
		sessionManager: sessionManager,
	}
}

func (h *ThirdPartyHandler) Auth(c echo.Context) error {
	errorRedirectTo := c.Request().Header.Get("Referer")
	if errorRedirectTo == "" {
		errorRedirectTo = h.cfg.ThirdParty.ErrorRedirectURL
	}

	var request dto.ThirdPartyAuthRequest
	err := c.Bind(&request)
	if err != nil {
		return h.redirectError(c, thirdparty.ErrorServer("could not decode request payload").WithCause(err), errorRedirectTo)
	}

	err = c.Validate(request)
	if err != nil {
		return h.redirectError(c, thirdparty.ErrorInvalidRequest(err.Error()).WithCause(err), errorRedirectTo)
	}

	if ok := thirdparty.IsAllowedRedirect(h.cfg.ThirdParty, request.RedirectTo); !ok {
		return h.redirectError(c, thirdparty.ErrorInvalidRequest(fmt.Sprintf("redirect to '%s' not allowed", request.RedirectTo)), errorRedirectTo)
	}

	errorRedirectTo = request.RedirectTo

	provider, err := thirdparty.GetProvider(h.cfg.ThirdParty, request.Provider)
	if err != nil {
		return h.redirectError(c, thirdparty.ErrorInvalidRequest(err.Error()).WithCause(err), errorRedirectTo)
	}

	state, err := thirdparty.GenerateState(h.cfg, provider.Name(), request.RedirectTo)
	if err != nil {
		return h.redirectError(c, thirdparty.ErrorServer("could not generate state").WithCause(err), errorRedirectTo)
	}

	authCodeUrl := provider.AuthCodeURL(string(state), oauth2.SetAuthURLParam("prompt", "consent"))

	c.SetCookie(&http.Cookie{
		Name:     HankoThirdpartyStateCookie,
		Value:    string(state),
		Path:     "/",
		Domain:   h.cfg.Session.Cookie.Domain,
		MaxAge:   300,
		Secure:   h.cfg.Session.Cookie.Secure,
		HttpOnly: h.cfg.Session.Cookie.HttpOnly,
		SameSite: http.SameSiteLaxMode,
	})

	return c.Redirect(http.StatusTemporaryRedirect, authCodeUrl)
}

func (h *ThirdPartyHandler) CallbackPost(c echo.Context) error {
	q, err := c.FormParams()
	if err != nil {
		return h.redirectError(c, thirdparty.ErrorServer("could not get form parameters"), h.cfg.ThirdParty.ErrorRedirectURL)
	}

	path := fmt.Sprintf("/thirdparty/callback?%s", q.Encode())
	pathPrefix := h.cfg.Server.Public.PathPrefix
	if len(pathPrefix) > 0 {
		path = pathPrefix + path
	}

	return c.Redirect(http.StatusSeeOther, path)
}

func (h *ThirdPartyHandler) Callback(c echo.Context) error {
	var successRedirectTo string
	var accountLinkingResult *thirdparty.AccountLinkingResult
	errorRedirectTo := h.cfg.ThirdParty.ErrorRedirectURL

	err := h.persister.Transaction(func(tx *pop.Connection) error {
		var callback dto.ThirdPartyAuthCallback
		terr := c.Bind(&callback)
		if terr != nil {
			return thirdparty.ErrorServer("could not decode request payload").WithCause(terr)
		}

		terr = c.Validate(callback)
		if terr != nil {
			if eerr, ok := terr.(*echo.HTTPError); ok {
				if message, ok2 := eerr.Message.(string); ok2 {
					return thirdparty.ErrorInvalidRequest(message).WithCause(terr)
				} else {
					return thirdparty.ErrorInvalidRequest(terr.Error()).WithCause(terr)
				}
			} else {
				return thirdparty.ErrorInvalidRequest(terr.Error()).WithCause(terr)
			}
		}

		state, terr := thirdparty.DecodeState(h.cfg, callback.State)
		if terr != nil {
			return thirdparty.ErrorInvalidRequest(terr.Error()).WithCause(terr)
		}

		redirectTo, terr := url.Parse(state.RedirectTo)
		if terr != nil {
			return thirdparty.ErrorServer("could not parse redirect url").WithCause(terr)
		}

		eRedirectTo, _ := url.Parse(redirectTo.String())
		eRedirectTo.RawQuery = ""
		errorRedirectTo = eRedirectTo.String()

		expectedState, terr := c.Cookie(HankoThirdpartyStateCookie)
		if terr != nil {

			return thirdparty.ErrorInvalidRequest("thirdparty state cookie is missing")
		}

		cookieState, terr := thirdparty.DecodeState(h.cfg, expectedState.Value)
		if terr != nil {
			return thirdparty.ErrorInvalidRequest(terr.Error()).WithCause(terr)
		}

		if cookieState.Nonce != state.Nonce {
			return thirdparty.ErrorInvalidRequest("nonce mismatch")
		}

		if callback.HasError() {
			return thirdparty.NewThirdPartyError(callback.Error, callback.ErrorDescription)
		}

		provider, terr := thirdparty.GetProvider(h.cfg.ThirdParty, state.Provider)
		if terr != nil {
			return thirdparty.ErrorInvalidRequest(terr.Error()).WithCause(terr)
		}

		if callback.AuthCode == "" {
			return thirdparty.ErrorInvalidRequest("auth code missing from request")
		}

		oAuthToken, terr := provider.GetOAuthToken(callback.AuthCode)
		if terr != nil {
			return thirdparty.ErrorInvalidRequest("could not exchange authorization code for access token").WithCause(terr)
		}

		userData, terr := provider.GetUserData(oAuthToken)
		if terr != nil {
			return thirdparty.ErrorInvalidRequest("could not retrieve user data from provider").WithCause(terr)
		}

		linkingResult, terr := thirdparty.LinkAccount(tx, h.cfg, h.persister, userData, provider.Name())
		if terr != nil {
			return terr
		}
		accountLinkingResult = linkingResult

		token, terr := models.NewToken(linkingResult.User.ID)
		if terr != nil {
			return thirdparty.ErrorServer("could not create token").WithCause(terr)
		}

		terr = h.persister.GetTokenPersisterWithConnection(tx).Create(*token)
		if terr != nil {
			return thirdparty.ErrorServer("could not save token to db").WithCause(terr)
		}

		query := redirectTo.Query()
		query.Add(HankoTokenQuery, token.Value)
		redirectTo.RawQuery = query.Encode()
		successRedirectTo = redirectTo.String()

		c.SetCookie(&http.Cookie{
			Name:     HankoThirdpartyStateCookie,
			Value:    "",
			Path:     "/",
			Domain:   h.cfg.Session.Cookie.Domain,
			MaxAge:   -1,
			Secure:   h.cfg.Session.Cookie.Secure,
			HttpOnly: h.cfg.Session.Cookie.HttpOnly,
			SameSite: http.SameSiteLaxMode,
		})

		return nil
	})

	if err != nil {
		return h.redirectError(c, err, errorRedirectTo)
	}

	err = h.auditLogger.Create(c, accountLinkingResult.Type, accountLinkingResult.User, nil)

	if err != nil {
		return h.redirectError(c, thirdparty.ErrorServer("could not create audit log").WithCause(err), errorRedirectTo)
	}

	return c.Redirect(http.StatusTemporaryRedirect, successRedirectTo)
}

func (h *ThirdPartyHandler) redirectError(c echo.Context, error error, to string) error {
	redirectTo := h.cfg.ThirdParty.ErrorRedirectURL
	if to != "" {
		redirectTo = to
	}

	err := h.auditError(c, error)
	if err != nil {
		error = err
	}

	redirectURL := thirdparty.GetErrorUrl(redirectTo, error)

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *ThirdPartyHandler) auditError(c echo.Context, err error) error {
	e, ok := err.(*thirdparty.ThirdPartyError)

	var auditLogError error
	if ok && e.Code != thirdparty.ErrorCodeServerError {
		auditLogError = h.auditLogger.Create(c, models.AuditLogThirdPartySignInSignUpFailed, nil, err)
	}
	return auditLogError
}
