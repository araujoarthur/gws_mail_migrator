package gapismanager

import "strings"

// API SCOPES
type GAPIScope string

const (
	GAPIEmptyScope       GAPIScope = ""
	GmailInsertScope     GAPIScope = "https://www.googleapis.com/auth/gmail.insert"
	GmailReadOnlyScope   GAPIScope = "https://www.googleapis.com/auth/gmail.readonly"
	GmailLabelsScope     GAPIScope = "https://www.googleapis.com/auth/gmail.labels"
	GmailModifyScope     GAPIScope = "https://www.googleapis.com/auth/gmail.modify"
	GroupsMigrationScope GAPIScope = "https://www.googleapis.com/auth/apps.groups.migration"
)

const scopeSeparator = " "

type GAPIScopeManager struct {
	scopes []GAPIScope
}

func NewEmptyScopeManager() *GAPIScopeManager {
	return &GAPIScopeManager{
		scopes: make([]GAPIScope, 0),
	}
}

func (s *GAPIScopeManager) Acquire(g GAPIScope) {
	s.scopes = append(s.scopes, g)
}

func (s *GAPIScopeManager) String() string {
	var b strings.Builder
	for i, scope := range s.scopes {
		if i > 0 {
			b.WriteString(scopeSeparator)
		}
		b.WriteString(string(scope))
	}

	return b.String()
}
