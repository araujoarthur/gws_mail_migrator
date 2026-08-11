package gapismanager

import (
	"encoding/json"
	"fmt"
	"os"
)

type SACredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokeURI                 string `json:"token_uri"`
	AuthProviderx509CertURL string `json:"auth_provider_x509_cert_url"`
	Clientx509CertUrl       string `json:"client_x509_cert_url"`
	UniverseDomain          string `json:"universe_domain"`
}

func LoadCredentialsFile(path string) (SACredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SACredentials{}, fmt.Errorf("read credentials: %w", err)
	}

	var credentials SACredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return SACredentials{}, fmt.Errorf("parse credentials: %w", err)
	}

	return credentials, nil
}

func (c *SACredentials) Validate() error {
	if c.PrivateKey == "" || c.ClientEmail == "" {
		return fmt.Errorf("credentials are missing required fields")
	}

	return nil
}
