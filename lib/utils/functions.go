package utils

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
)

func FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func FolderExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func ListEmailFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.EqualFold(filepath.Ext(entry.Name()), ".eml") {
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}

	return files, nil
}

func ReadEmailFile(path string) (EmailMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return EmailMetadata{}, err
	}
	defer f.Close()

	filehash, err := HashEmailFile(f)
	if err != nil {
		return EmailMetadata{}, fmt.Errorf("hashing file: %w", err)
	}

	msg, err := mail.ReadMessage(f)

	if err != nil {
		return EmailMetadata{}, err
	}

	from, err := mail.ParseAddress(msg.Header.Get("From"))
	if err != nil {
		return EmailMetadata{}, fmt.Errorf("invalid From header: %w", err)
	}

	emailDate, err := msg.Header.Date()
	if err != nil {
		return EmailMetadata{}, err
	}

	messageID := strings.TrimSpace(msg.Header.Get("Message-ID"))
	if messageID == "" {
		return EmailMetadata{}, errors.New("email has no Message-ID header")
	}

	decoder := new(mime.WordDecoder)

	subject, err := decoder.DecodeHeader(msg.Header.Get("Subject"))
	if err != nil {
		return EmailMetadata{}, err
	}

	return EmailMetadata{
		MessageID: messageID,
		Filename:  path,
		Sender:    from.Address,
		Date:      emailDate,
		FileHash:  filehash,
		Subject:   subject,
	}, nil
}

func hashSHA256(reader io.Reader) ([]byte, error) {
	hasher := sha256.New()

	if _, err := io.Copy(hasher, reader); err != nil {
		return nil, fmt.Errorf("calculate SHA-256: %w", err)
	}

	return hasher.Sum(nil), nil
}

func HashEmailFile(file *os.File) ([]byte, error) {
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("get file position: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to beginning: %w", err)
	}

	hash, hashErr := hashSHA256(file)

	_, seekErr := file.Seek(position, io.SeekStart)

	if hashErr != nil {
		return nil, hashErr
	}
	if seekErr != nil {
		return nil, fmt.Errorf("restore file position: %w", seekErr)
	}

	return hash, nil
}

func ParseSQLiteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)

	parts := strings.Fields(value)
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		previous := parts[len(parts)-2]

		if last == previous && isNumericTimezone(last) {
			value = strings.Join(parts[:len(parts)-1], " ")
		}
	}

	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf(
		"unsupported date format %q",
		value,
	)
}

func isNumericTimezone(value string) bool {
	if len(value) != 5 {
		return false
	}

	if value[0] != '+' && value[0] != '-' {
		return false
	}

	for _, character := range value[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}

func generateRunID() uuid.UUID {
	runUUID, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Errorf("generate run uuid: %w", err))
	}

	return runUUID
}

var runUniqueID = generateRunID()

func GetRunID() string {
	return runUniqueID.String()
}
