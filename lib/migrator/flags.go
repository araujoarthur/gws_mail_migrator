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
	MigrationFlagTargetUser
	MigrationFlagTargetGroup
	MigrationFlagDryRun
)

func (m MigrationFlag) Has(mf MigrationFlag) bool {
	return m&mf != 0
}

func (m MigrationFlag) Validate() error {
	MigrationModes := MigrationFlagModeDelta | MigrationFlagModeStandard
	if m.Has(MigrationModes) {
		return fmt.Errorf("flags 'MigrationFlagModeDelta' and 'MigrationFlagModeStandard' cannot coexist")
	}

	MigrationOrdering := MigrationFlagOrderNewestFirst | MigrationFlagOrderOldestFirst
	if m.Has(MigrationOrdering) {
		return fmt.Errorf("flags 'MigrationFlagOrderNewestFirst' and 'MigrationFlagOldestFirst' cannot coexist")
	}

	MigrationTargets := MigrationFlagTargetGroup | MigrationFlagTargetUser
	if m.Has(MigrationTargets) {
		return fmt.Errorf("flags 'MigrationFlagTargetGroup' and 'MigrationFlagTargetUser' cannot coexist")
	}

	return nil
}

func (m MigrationFlag) GetOrderString() string {
	if m.Has(MigrationFlagOrderNewestFirst) {
		return "DESC"
	}

	return "ASC"
}

func (m MigrationFlag) GetTargetTypeString() string {
	if m.Has(MigrationFlagTargetUser) {
		return "u"
	}

	return "g"
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
