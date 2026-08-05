package impersonator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	GmailInsertScope   = "https://www.googleapis.com/auth/gmail.insert"
	GmailReadOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"
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

type Impersonator struct {
	tokenManager TokenManager
	//emlFilesFolderPath string
	httpClient *http.Client
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

func NewImpersonator(credentialsPath string, targetUser string, targetScopes string) (*Impersonator, error) {
	credentials, err := LoadCredentialsFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("loading credentials file: %w", err)
	}

	return &Impersonator{
		tokenManager: TokenManager{
			renewChan:   make(chan struct{}),
			targetUser:  targetUser,
			scopes:      targetScopes,
			credentials: credentials,
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *SACredentials) Validate() error {
	if c.PrivateKey == "" || c.ClientEmail == "" {
		return fmt.Errorf("credentials are missing required fields")
	}

	return nil
}

type InsertResult struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	LabelIDs []string `json:"labelIds"`
}

func (imp *Impersonator) InsertRawEML(
	ctx context.Context,
	content io.Reader,
) (InsertResult, error) {
	validToken, err := imp.tokenManager.GetValidToken(ctx)
	if err != nil {
		return InsertResult{}, fmt.Errorf(
			"get valid token: %w",
			err,
		)
	}

	body, contentType := createMultipartBody(content, []string{"INBOX", "UNREAD"})
	defer body.Close()

	const apiURL = "https://gmail.googleapis.com/upload/gmail/v1/users/me/messages" +
		"?uploadType=multipart&internalDateSource=dateHeader"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		body,
	)
	if err != nil {
		return InsertResult{}, fmt.Errorf(
			"create Gmail request: %w",
			err,
		)
	}

	req.Header.Set("Authorization", "Bearer "+validToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := imp.httpClient.Do(req)
	if err != nil {
		return InsertResult{}, fmt.Errorf(
			"execute Gmail request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 1<<20),
		)
		if readErr != nil {
			return InsertResult{}, fmt.Errorf(
				"Gmail API returned %s; read response: %w",
				resp.Status,
				readErr,
			)
		}

		return InsertResult{}, fmt.Errorf(
			"Gmail API returned %s: %s",
			resp.Status,
			strings.TrimSpace(string(body)),
		)
	}

	var result InsertResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return InsertResult{}, fmt.Errorf(
			"decode Gmail insert response: %w",
			err,
		)
	}

	if result.ID == "" {
		return InsertResult{}, errors.New(
			"Gmail insert returned no message ID",
		)
	}

	return result, nil
}
