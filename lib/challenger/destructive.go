package challenger

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type DestructiveAction string

const (
	ActionResetDatabase DestructiveAction = "reset_db"
)

type DestructiveChallenge struct {
	Code      string            `json:"code"`
	Action    DestructiveAction `json:"action"`
	CreatedAt time.Time         `json:"created_at"`
}

func (c DestructiveChallenge) Validate(provided string) error {
	if provided != c.Code {
		return errors.New("invalid destructive confirmation code")
	}

	// One-time use.
	return c.Remove()
}

func (c DestructiveChallenge) Remove() error {
	path, err := buildDestructiveChallengePath(c.Action)
	if err != nil {
		return fmt.Errorf("could not build challenge path: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("could not consume challenge: %w", err)
	}

	return nil
}

func NewDestructiveChallenge(action DestructiveAction) (DestructiveChallenge, error) {
	return createDestructiveChallenge(action)
}

func LoadDestructiveChallenge(action DestructiveAction) (DestructiveChallenge, error) {
	path, err := buildDestructiveChallengePath(action)
	if err != nil {
		return DestructiveChallenge{}, fmt.Errorf(
			"could not build challenge path: %w",
			err,
		)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DestructiveChallenge{}, errors.New(
				"no destructive challenge exists",
			)
		}

		return DestructiveChallenge{}, fmt.Errorf(
			"could not read challenge: %w",
			err,
		)
	}

	var challenge DestructiveChallenge
	if err := json.Unmarshal(data, &challenge); err != nil {
		return DestructiveChallenge{}, fmt.Errorf("invalid destructive challenge: %w", err)
	}

	if challenge.Action != action {
		return DestructiveChallenge{}, errors.New(
			"destructive challenge action mismatch",
		)
	}

	if time.Since(challenge.CreatedAt) > 5*time.Minute {
		_ = os.Remove(path)

		return DestructiveChallenge{}, errors.New(
			"destructive challenge confirmation code has expired",
		)
	}

	return challenge, nil
}

func generateAckCode() (string, error) {
	const digits = "0123456789"

	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))

		if err != nil {
			return "", err
		}

		code[i] = digits[n.Int64()]
	}

	return string(code), nil
}

func buildDestructiveChallengePath(action DestructiveAction) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir = filepath.Join(dir, "gmm")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("%s-challenge.json", string(action))), nil
}

func createDestructiveChallenge(action DestructiveAction) (DestructiveChallenge, error) {
	code, err := generateAckCode()
	if err != nil {
		return DestructiveChallenge{}, err
	}

	challenge := DestructiveChallenge{
		Code:      code,
		Action:    action,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(challenge)
	if err != nil {
		return DestructiveChallenge{}, err
	}

	path, err := buildDestructiveChallengePath(action)
	if err != nil {
		return DestructiveChallenge{}, err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return DestructiveChallenge{}, err
	}

	return challenge, nil

}
