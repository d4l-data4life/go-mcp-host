package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

var (
	ErrNoKeyRegistry = errors.New("no remote key registry configured")
)

type TokenValidator interface {
	ValidateJWT(token string) (*jwt.Token, error)
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
