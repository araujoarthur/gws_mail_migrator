package mailinserter

import (
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

func (u *UserMailInserter) InsertRawEML(
	ctx context.Context,
	content io.Reader,
) (InsertResult, error) {
	validToken, err := u.tokenManager.GetValidToken(ctx)
	if err != nil {
		return InsertResult{}, fmt.Errorf(
			"get valid token: %w",
			err,
		)
	}

	body, contentType := gapismanager.CreateMultipartBody(content, []string{"INBOX", "UNREAD"})
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
