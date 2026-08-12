package migrator

import (
	"fmt"
	"sort"
	"strings"
)

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

var flagNames = map[MigrationFlag]string{
	MigrationFlagModeStandard:     "mode_standard",
	MigrationFlagModeDelta:        "mode_delta",
	MigrationFlagOrderNewestFirst: "order_newest_first",
	MigrationFlagOrderOldestFirst: "order_oldest_first",
	MigrationFlagLocalEnforce:     "local_enforce",
	MigrationFlagTargetUser:       "target_user",
	MigrationFlagTargetGroup:      "target_group",
	MigrationFlagDryRun:           "dry_run",
}

func (m MigrationFlag) String() string {
	if name, ok := flagNames[m]; ok {
		return name
	}

	return fmt.Sprintf("MigrationFlag(%#x)", uint64(m))
}

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

func (m MigrationFlag) Names() []string {
	names := make([]string, 0, len(flagNames))

	for flag, name := range flagNames {
		if m.Has(flag) {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names
}

func (m MigrationFlag) NamesString() string {
	return strings.Join(m.Names(), ", ")
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
