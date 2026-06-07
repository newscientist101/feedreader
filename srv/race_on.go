//go:build race

package srv

// raceEnabled reports whether the binary was built with the race detector
// (-race). The detector instruments every memory access and adds large CPU
// overhead, so wall-clock latency assertions (see TestPerformance) are
// meaningless under it and are skipped.
const raceEnabled = true
