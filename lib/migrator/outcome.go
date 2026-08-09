package migrator

type MigrationOutcome uint8

const (
	MigrationOutcomeUnknown MigrationOutcome = iota
	MigrationOutcomeInserted
	MigrationOutcomeAlreadyExists
)

func (m MigrationOutcome) StringRepr() string {
	switch m {
	case MigrationOutcomeUnknown:
		return "unknown"
	case MigrationOutcomeInserted:
		return "inserted"
	case MigrationOutcomeAlreadyExists:
		return "already_exists"
	default:
		return "fault_unknown"
	}
}
