package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

var (
	ErrNoKeyRegistry   = errors.New("no remote key registry configured")
	ErrInvalidJWTKey   = errors.New("invalid JWT key")
	ErrTokenValidation = errors.New("token validation failed")
)

// TokenValidator defines the interface for validating JWT tokens
type TokenValidator interface {
	ValidateJWT(token string) (*jwt.Token, error)
}

// LocalJWTValidator validates JWTs signed with a local symmetric key
type LocalJWTValidator struct {
	jwtSecret []byte
}

// NewLocalJWTValidator creates a new local JWT validator with the provided signing key
func NewLocalJWTValidator(jwtSecret []byte) (*LocalJWTValidator, error) {
	if len(jwtSecret) == 0 {
		return nil, ErrInvalidJWTKey
	}
	return &LocalJWTValidator{
		jwtSecret: jwtSecret,
	}, nil
}

// ValidateJWT validates a JWT token signed with the local key
func (v *LocalJWTValidator) ValidateJWT(token string) (*jwt.Token, error) {
	t, err := jwt.Parse(
		[]byte(token),
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.HS256, v.jwtSecret),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenValidation, err)
	}
	return &t, nil
}

type RemoteKeyStore struct {
	keyStore *jwk.AutoRefresh
	uri      string
}

func NewRemoteKeyStore(ctx context.Context, uri string) (*RemoteKeyStore, error) {
	logging.LogInfofCtx(ctx, "attempting to create remote Key Store.")
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "https" {
		return nil, fmt.Errorf("key store URL must use HTTPS protocol")
	}

	ks := RemoteKeyStore{
		keyStore: jwk.NewAutoRefresh(ctx),
		uri:      uri,
	}

	ks.keyStore.Configure(ks.uri)

	set, err := ks.keyStore.Refresh(ctx, ks.uri)
	if err != nil {
		return nil, err
	}

	logging.LogInfofCtx(ctx, "remote Key Store initialized. # of retrieved keys: %d", set.Len())

	return &ks, nil
}

func (ks *RemoteKeyStore) ValidateJWT(token string) (*jwt.Token, error) {
	var t jwt.Token

	if ks.keyStore == nil {
		return nil, ErrNoKeyRegistry
	}

	// Fetch will honor all HTTP cache headers that may be sent by the keys endpoint.
	// That is, we do not do an HTTP request each time!
	set, err := ks.keyStore.Fetch(context.Background(), ks.uri)
	if err != nil {
		return nil, err
	}

	t, err = jwt.Parse([]byte(token),
		jwt.WithValidate(true),
		jwt.InferAlgorithmFromKey(true),
		jwt.WithKeySet(set))
	if err == nil {
		return &t, nil
	}

	return nil, err
}
