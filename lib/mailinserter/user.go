package mailinserter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/araujoarthur/gws_mail_migrator/lib/gapismanager"
)

type UserMailInserter struct {
	userAddress  string
	tokenManager *gapismanager.TokenManager
	httpClient   *http.Client
	logger       *slog.Logger
}

func NewUserMailInserter(user string, logger *slog.Logger) (*UserMailInserter, error) {
	scopes := gapismanager.NewEmptyScopeManager()
	scopes.Acquire(gapismanager.GmailInsertScope)
	scopes.Acquire(gapismanager.GmailReadOnlyScope)
	scopes.Acquire(gapismanager.GmailLabelsScope)

	userMailInserterLogger := logger.With("component", "user_inserter", "user_address", user)

	tokenManager, err := gapismanager.NewTokenManager(user, scopes.String(), userMailInserterLogger)
	if err != nil {
		userMailInserterLogger.Error("failed to create a token manager for user mail inserter", "error", err)
		return nil, err
	}

	return &UserMailInserter{
		userAddress:  user,
		tokenManager: tokenManager,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       userMailInserterLogger,
	}, nil
}

func (u *UserMailInserter) EmailExists(ctx context.Context, messageID string) (bool, error) {
	token, err := u.tokenManager.GetValidToken(ctx)
	if err != nil {
		u.logger.Error("failed to check email existence", "step", "get_valid_token", "error", err)
		return false, fmt.Errorf("get valid token: %w", err)
	}

	query := "rfc822msgid:" + messageID

	params := url.Values{}
	params.Set("q", query)
	params.Set("maxResults", "1")
	params.Set("includeSpamTrash", "true")

	apiURL := "https://gmail.googleapis.com/gmail/v1/users/me/messages?" + params.Encode()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		apiURL,
		nil,
	)

	if err != nil {
		u.logger.Error("failed to check email existence", "step", "new_request_with_context", "error", err)
		return false, fmt.Errorf("create message search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := u.httpClient.Do(req)
	if err != nil {
		u.logger.Error("failed to check email existence", "step", "do_request", "error", err)
		return false, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		u.logger.Error("failed to check email existence", "step", "check_response", "response_status", resp.Status, "body", strings.TrimSpace(string(body)))
		return false, fmt.Errorf("Gmail search returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		u.logger.Error("failed to check email existence", "step", "decode_gmail_response", "error", err)
		return false, fmt.Errorf("decode Gmail search response: %w", err)
	}

	return len(result.Messages) > 0, nil
}

func (u *UserMailInserter) InsertRawEML(ctx context.Context, content io.Reader, labelIds []string) (InsertResult, error) {
	validToken, err := u.tokenManager.GetValidToken(ctx)
	if err != nil {
		return InsertResult{}, fmt.Errorf(
			"get valid token: %w",
			err,
		)
	}

	if labelIds == nil {
		labelIds = []string{"INBOX", "UNREAD"}
	}

	body, contentType := gapismanager.CreateMultipartBody(content, labelIds)
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
		return InsertResult{}, fmt.Errorf("create Gmail request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+validToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return InsertResult{}, fmt.Errorf("execute Gmail request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 1<<20),
		)
		if readErr != nil {
			return InsertResult{}, fmt.Errorf("Gmail API returned %s; read response: %w", resp.Status, readErr)
		}

		return InsertResult{}, fmt.Errorf("Gmail API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result InsertResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return InsertResult{}, fmt.Errorf("decode Gmail insert response: %w", err)
	}

	if result.ID == "" {
		return InsertResult{}, errors.New("Gmail insert returned no message ID")
	}

	return result, nil
}

func (u *UserMailInserter) GetLabels(ctx context.Context) ([]GmailLabel, error) {
	token, err := u.tokenManager.GetValidToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get valid token: %w", err)
	}

	const apiURL = "https://gmail.googleapis.com/gmail/v1/users/me/labels"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create list-labels request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute list-labels request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return nil, fmt.Errorf("Gmail API returned %s; read response: %w", resp.Status, err)
		}

		return nil, fmt.Errorf("Gmail API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result struct {
		Labels []GmailLabel `json:"labels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Gmail labels response: %w", err)
	}

	return result.Labels, nil

}

func (u *UserMailInserter) FindLabel(ctx context.Context, name string) (GmailLabel, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GmailLabel{}, false, errors.New("label name cannot be empty")
	}

	result, err := u.GetLabels(ctx)
	if err != nil {
		return GmailLabel{}, false, fmt.Errorf("get labels: %w", err)
	}
	for _, label := range result {
		if label.Name == name {
			return label, true, nil
		}
	}

	return GmailLabel{}, false, nil
}

func (u *UserMailInserter) CreateLabel(ctx context.Context, name string) (GmailLabel, error) {
	if name == "" {
		return GmailLabel{}, errors.New("label name cannot be empty")
	}

	token, err := u.tokenManager.GetValidToken(ctx)
	if err != nil {
		return GmailLabel{}, fmt.Errorf("get valid token: %w", err)
	}

	requestBody := struct {
		Name                  string `json:"name"`
		LabelListVisibility   string `json:"labelListVisibility"`
		MessageListVisibility string `json:"messageListVisibility"`
	}{
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(requestBody); err != nil {
		return GmailLabel{}, fmt.Errorf("encode create-label request: %w", err)
	}

	const apiURL = "https://gmail.googleapis.com/gmail/v1/users/me/labels"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &body)
	if err != nil {
		return GmailLabel{}, fmt.Errorf("create Gmail label request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return GmailLabel{}, fmt.Errorf("execute Gmail label request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return GmailLabel{}, fmt.Errorf("Gmail API returned %s; read response: %w", resp.Status, readErr)
		}

		return GmailLabel{}, fmt.Errorf("Gmail API returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}

	var label GmailLabel
	if err := json.NewDecoder(resp.Body).Decode(&label); err != nil {
		return GmailLabel{}, fmt.Errorf("decode created Gmail label: %w", err)
	}

	if label.ID == "" {
		return GmailLabel{}, errors.New("Gmail label creation returned no label ID")
	}

	u.logger.Info(
		"created Gmail label",
		"label_id", label.ID,
		"label_name", label.Name,
	)

	return label, nil
}
