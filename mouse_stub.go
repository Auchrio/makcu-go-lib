//go:build !windows

package Macku

import "errors"

// MoveAbs is only supported on Windows where GetCursorPos and
// SystemParametersInfoW are available.
func (m *Mouse) MoveAbs(target [2]int, speed int, waitMs int) error {
	return errors.New("MoveAbs is only supported on Windows")
}
