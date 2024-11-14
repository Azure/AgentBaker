package ado

import "fmt"

type ErrNoBuildsFound struct {
	DefinitionID int
	BranchName   string
}

func (e *ErrNoBuildsFound) Error() string {
	return fmt.Sprintf("no matching builds found for definition %d off branch %s", e.DefinitionID, e.BranchName)
}
