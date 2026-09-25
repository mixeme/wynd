package version

// Number is the release semver. Keep identical to the VERSION file
// at the repository root (enforced by TestMatchesVERSIONFile).
const Number = "0.7.0"

// String returns the current semver. It does not read the working
// directory: an installed binary has no VERSION file next to CWD.
func String() string {
	return Number
}
