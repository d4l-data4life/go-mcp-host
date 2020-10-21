package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// RequesterContext is the key for the requester account ID in chi context
const RequesterContext contextKey = "accountID"

type claims struct {
	jwt.StandardClaims

	// ClientID is the Client ID claim (gesundheitscloud private claim)
	ClientID string `json:"ghc:cid"`

	// UserID is the claim that encodes the user who originally requested the JWT (gesundheitscloud private claim)
	UserID uuid.UUID `json:"ghc:uid"`
}

// NewAuthMiddleware initializes a new handler
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{
		Handler:        GetHandlerFactory().NewHandler("AuthMiddleware"),
		authSecret:     viper.GetString("SERVICE_SECRET"),
		authHeaderName: viper.GetString("AUTH_HEADER_NAME"),
	}
}

// AuthMiddleware is the handler responsible for Account operations
type AuthMiddleware struct {
	*instrumented.Handler
	authSecret     string
	authHeaderName string
}

// ServiceSecretMiddleware serves requests with valid service secret
func (auth *AuthMiddleware) ServiceSecretMiddleware(next http.Handler) http.Handler {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken, err := auth.GetAuthSecret(r)
		if err != nil {
			WriteHTTPErrorCode(w, err, http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(authToken), []byte(auth.authSecret)) == 0 {
			WriteHTTPErrorCode(w, errors.New("invalid authorization secret"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
	return auth.Instrumenter().Instrument("auth", handlerFunc, config.DefaultInstrumentOptions...)
}

// UserAuthMiddleware serves requests with valid service secret
func (auth *AuthMiddleware) UserAuthMiddleware(next http.Handler) http.Handler {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken, err := auth.GetBearerToken(w, r)
		if err != nil {
			WriteHTTPErrorCode(w, err, http.StatusUnauthorized)
		}
		tk := &claims{}
		_, err = jwt.ParseWithClaims(authToken, tk, func(token *jwt.Token) (interface{}, error) {
			return config.PublicKey, nil
		})

		if err != nil {
			if ve, ok := err.(*jwt.ValidationError); ok {
				switch {
				case ve.Errors&jwt.ValidationErrorMalformed != 0:
					logging.LogError("malformed jwt", err)
					WriteHTTPErrorCode(w, errors.New("token is malformed"), http.StatusUnauthorized)
					return
				case ve.Errors&jwt.ValidationErrorExpired != 0:
					WriteHTTPErrorCode(w, errors.New("token is expired"), http.StatusUnauthorized)
					return
				case ve.Errors&jwt.ValidationErrorNotValidYet != 0:
					WriteHTTPErrorCode(w, errors.New("token is not valid yet"), http.StatusUnauthorized)
					return
				default:
					logging.LogError("Error parsing jwt", err)
					WriteHTTPErrorCode(w, errors.New("error parsing jwt"), http.StatusUnauthorized)
					return
				}
			}
		}
		accountID := strings.Split(tk.Subject, ":")[1]
		logging.LogInfof("Setting context %s value %s", RequesterContext, accountID)
		ctx := context.WithValue(r.Context(), RequesterContext, accountID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
	return auth.Instrumenter().Instrument("auth", handlerFunc, config.DefaultInstrumentOptions...)
}

// GetAuthSecret returns the contents of the authorization header
func (auth *AuthMiddleware) GetAuthSecret(r *http.Request) (string, error) {
	authHeaderContent := r.Header.Get(auth.authHeaderName)
	if authHeaderContent == "" {
		err := errors.New("missing authentication header")
		logging.LogErrorf(err, "error in secret-based authorization")
		return "", err
	}
	return authHeaderContent, nil
}

// GetBearerToken returns the Bearer AuthToken from the given request
func (auth *AuthMiddleware) GetBearerToken(w http.ResponseWriter, r *http.Request) (string, error) {
	authHeaderContent, err := auth.GetAuthSecret(r)
	if err != nil {
		return "", err
	}

	authTokenSplit := strings.Split(authHeaderContent, "Bearer ")
	if len(authTokenSplit) != 2 {
		return "", errors.New("malformed authentication header")
	}

	authToken := authTokenSplit[1]
	if authToken == "" {
		return "", errors.New("malformed authentication header")
	}
	return authToken, nil
}

//ParseRequesterID returns the requester account id from context (only for protected endpoints)
func ParseRequesterID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	requester := r.Context().Value(RequesterContext)
	if requester == nil {
		err := errors.New("missing account id")
		logging.LogError("error parsing Requester UUID", err)
		WriteHTTPErrorCode(w, err, http.StatusBadRequest)
		return uuid.Nil, err
	}

	requesterID, err := uuid.FromString(requester.(string))
	if err != nil || requesterID == uuid.Nil {
		logging.LogError("error parsing Requester UUID", err)
		WriteHTTPErrorCode(w, errors.New("malformed Account ID"), http.StatusBadRequest)
		return uuid.Nil, err
	}
	return requesterID, nil
}
