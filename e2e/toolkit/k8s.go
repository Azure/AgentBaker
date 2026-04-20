package toolkit

import (
	"github.com/Masterminds/semver"
)

func CheckK8sConstraint(kubernetesVersion string, constraintStr string) (bool, error) {
	version, err := semver.NewVersion(kubernetesVersion)
	if err != nil {
		return false, err
	}
	constraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		return false, err
	}
	return constraint.Check(version), nil
}
