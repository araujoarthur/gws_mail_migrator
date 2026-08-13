package utils

import "time"

type EmailMetadata struct {
	MessageID string
	Filename  string
	Sender    string
	Subject   string
	FileHash  []byte
	LabelIDs  []string
	Date      time.Time
}
