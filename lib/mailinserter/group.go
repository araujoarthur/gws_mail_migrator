package mailinserter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/araujoarthur/gws_mail_migrator/lib/gapismanager"
)

type GroupMailInserter struct {
	groupAddress string
	tokenManager *gapismanager.TokenManager
	httpClient   *http.Client
	logger       *slog.Logger
}

func NewGroupMailInserter(groupAddress string, systemUserAddress string, logger *slog.Logger) (*GroupMailInserter, error) {
	scopes := gapismanager.NewEmptyScopeManager()
	scopes.Acquire(gapismanager.GroupsMigrationScope)

	groupMailInserterLogger := logger.With("component", "group_inserter", "group_address", groupAddress)
	tokenManager, err := gapismanager.NewTokenManager(systemUserAddress, scopes.String(), groupMailInserterLogger)
	if err != nil {
		groupMailInserterLogger.Error("failed to create a token manager for group mail inserter", "error", err)
		return nil, err
	}

	return &GroupMailInserter{
		groupAddress: groupAddress,
		tokenManager: tokenManager,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       groupMailInserterLogger,
	}, nil
}

// InsertRawEML inserts an email in the archive of a group. The labelIds parameter exists to satisfy the interface but is effectively ignored
// in this implementation
func (g *GroupMailInserter) InsertRawEML(ctx context.Context, content io.Reader, labelIds []string) (InsertResult, error) {
	validToken, err := g.tokenManager.GetValidToken(ctx)
	if err != nil {
		g.logger.Error("failed to get valid token for groups migration", "error", err)
		return InsertResult{}, fmt.Errorf("get Groups Migration token: %w", err)
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/upload/groups/v1/groups/%s/archive?uploadType=media", url.PathEscape(g.groupAddress))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, content)
	if err != nil {
		g.logger.Error("failed to create group insertion request", "error", err)
		return InsertResult{}, fmt.Errorf("create group insertion request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+validToken)
	req.Header.Set("Content-Type", gapismanager.ContentMessage)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.logger.Error("failed to execute request", "error", err)
		return InsertResult{}, fmt.Errorf("insert email into group: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			g.logger.Error("failed to read response body", "error", err)
			return InsertResult{}, fmt.Errorf("Groups Migration API returned %s; read response: %w", resp.Status, readErr)
		}

		return InsertResult{}, fmt.Errorf("Groups Migration API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var response struct {
		Kind         string `json:"kind"`
		ResponseCode string `json:"responseCode"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return InsertResult{}, fmt.Errorf("decode Groups Migration response: %w", err)
	}

	if response.ResponseCode != "SUCCESS" {
		return InsertResult{}, fmt.Errorf("Groups Migration API returned response code %q", response.ResponseCode)
	}

	return InsertResult{
		ResponseCode: response.ResponseCode,
	}, nil
}
