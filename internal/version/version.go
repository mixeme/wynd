package version

// Number is the release semver. Keep identical to the VERSION file
// at the repository root (enforced by TestMatchesVERSIONFile).
const Number = "0.22.2"

// SourceURL is the public address of the Corresponding Source (AGPL §13).
//
// Единственное место, где записан адрес: клиент получает его полем source_url
// в GET /api/v1/instance, форку достаточно поменять эту строку (LIC-2).
// Публичный адрес — зеркало на GitHub, рабочий remote остаётся на Gitea.
const SourceURL = "https://github.com/mixeme/wynd"

// String returns the current semver. It does not read the working
// directory: an installed binary has no VERSION file next to CWD.
func String() string {
	return Number
}
