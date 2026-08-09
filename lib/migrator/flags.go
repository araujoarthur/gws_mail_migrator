package migrator

import "fmt"

type MigrationFlag int

const MigrationFlagEmpty MigrationFlag = 0

const (
	MigrationFlagModeStandard MigrationFlag = 1 << iota
	MigrationFlagModeDelta
	MigrationFlagOrderNewestFirst
	MigrationFlagOrderOldestFirst
	MigrationFlagLocalEnforce
)

func (m MigrationFlag) Has(mf MigrationFlag) bool {
	return m&mf != 0
}

func (m MigrationFlag) Validate() error {
	if m.Has(MigrationFlagModeDelta) && m.Has(MigrationFlagModeStandard) {
		return fmt.Errorf("flags MigrationFlagModeDelta and MigrationFlagModeStandard cannot coexist")
	}

	if m.Has(MigrationFlagOrderNewestFirst) && m.Has(MigrationFlagOrderOldestFirst) {
		return fmt.Errorf("flags MigrationFlagOrderNewestFirst and MigrationFlagOldestFirst cannot coexist")
	}

	return nil
}

func (m MigrationFlag) GetOrderString() string {
	if m.Has(MigrationFlagOrderNewestFirst) {
		return "DESC"
	}

	return "ASC"
}

// Set performs a destructive set on the given flag or flag field
func (m *MigrationFlag) Set(f MigrationFlag) {
	*m |= f
}

func (m *MigrationFlag) SetN(flags ...MigrationFlag) {
	for _, flag := range flags {
		m.Set(flag)
	}
}

// Unset performs a destructive unset on the given flag or flag field
func (m *MigrationFlag) Unset(f MigrationFlag) {
	*m &^= f
}

// WithSet performs a conservative set on the given flag or flag field
func (m MigrationFlag) WithSet(f MigrationFlag) MigrationFlag {
	return m | f
}

// WithUnset performs a conservative unset on the given flag or flag field
func (m MigrationFlag) WithUnset(f MigrationFlag) MigrationFlag {
	return m &^ f
}
