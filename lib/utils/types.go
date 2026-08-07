package utils

import "time"

type EmailMetadata struct {
	MessageID string
	Filename  string
	Sender    string
	Subject   string
	FileHash  []byte
	Date      time.Time
}
