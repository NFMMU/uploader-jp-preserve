package oidc

import (
	"encoding/json"
	"net/http"
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
	IdTokenSigningAlgValuesSupported string   `json:"id_token_signing_alg_values_supported" binding:"required"`
}

func OidcDiscovery(url string) (*OidcDiscoveryData, error) {
	resp, err := http.Get(url)
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
