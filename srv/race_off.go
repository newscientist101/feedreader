//go:build !race

package srv

// raceEnabled reports whether the binary was built with the race detector
// (-race). See race_on.go for details.
const raceEnabled = false
