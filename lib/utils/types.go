package utils

import "time"

type EmailMetadata struct {
	Filename string
	Sender   string
	Subject  string
	FileHash []byte
	Date     time.Time
}
