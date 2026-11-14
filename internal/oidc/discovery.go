package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type OidcDiscoveryData struct {
	Issuer                           string   `json:"issuer" binding:"required"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint" binding:"required"`
	TokenEndpoint                    string   `json:"token_endpoint" binding:"required"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint" binding:"omitempty"`
	ScopesSupported                  []string `json:"scopes_supported" binding:"omitempty"`
	JwksURI                          string   `json:"jwks_uri" binding:"required"`
	ResponseTypesSupported           []string `json:"response_types_supported" binding:"required"`
	SubjectTypesSupported            []string `json:"subject_types_supported" binding:"required"`
	IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported" binding:"required"`
}

func OidcDiscovery(ctx context.Context, url string) (*OidcDiscoveryData, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data OidcDiscoveryData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}
