package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nfmmu/uploader-archive/internal/gen/model"
)

func TestOidcProviderJSONOmitsClientSecret(t *testing.T) {
	provider := model.OidcProvider{ClientSecret: "private-value"}
	for _, value := range []any{provider, []*model.OidcProvider{&provider}} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "client_secret") || strings.Contains(string(data), "private-value") {
			t.Fatal("client secret exposed in JSON")
		}
	}
}
