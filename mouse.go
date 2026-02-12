package Macku

import (
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial/enumerator"
)

// Pre-built command strings for press/release (avoids fmt.Sprintf per call).
var (
	pressCommands   = [5]string{"km.left(1)", "km.right(1)", "km.middle(1)", "km.ms1(1)", "km.ms2(1)"}
	releaseCommands = [5]string{"km.left(0)", "km.right(0)", "km.middle(0)", "km.ms1(0)", "km.ms2(0)"}
)

// lockInfo holds the serial commands and cache bit for one lock target.
type lockInfo struct {
	lockCmd   string
	unlockCmd string
	queryCmd  string
	bit       int
}

var lockTargets = map[string]lockInfo{
	"LEFT":   {lockCmd: "km.lock_ml(1)", unlockCmd: "km.lock_ml(0)", queryCmd: "km.lock_ml()", bit: 0},
	"RIGHT":  {lockCmd: "km.lock_mr(1)", unlockCmd: "km.lock_mr(0)", queryCmd: "km.lock_mr()", bit: 1},
	"MIDDLE": {lockCmd: "km.lock_mm(1)", unlockCmd: "km.lock_mm(0)", queryCmd: "km.lock_mm()", bit: 2},
	"MOUSE4": {lockCmd: "km.lock_ms1(1)", unlockCmd: "km.lock_ms1(0)", queryCmd: "km.lock_ms1()", bit: 3},
	"MOUSE5": {lockCmd: "km.lock_ms2(1)", unlockCmd: "km.lock_ms2(0)", queryCmd: "km.lock_ms2()", bit: 4},
	"X":      {lockCmd: "km.lock_mx(1)", unlockCmd: "km.lock_mx(0)", queryCmd: "km.lock_mx()", bit: 5},
	"Y":      {lockCmd: "km.lock_my(1)", unlockCmd: "km.lock_my(0)", queryCmd: "km.lock_my()", bit: 6},
}

// DeviceInfo holds information about the connected Makcu device.
type DeviceInfo struct {
	Port        string
	Description string
	VID         string
	PID         string
	IsConnected bool
}

// Mouse provides mid-level mouse operations over the SerialTransport.
type Mouse struct {
	transport       *SerialTransport
	lockStatesCache int
	cacheValid      bool
}

// NewMouse creates a new Mouse bound to the given transport.
func NewMouse(transport *SerialTransport) *Mouse {
	return &Mouse{transport: transport}
}

// Press sends a button-press command.
func (m *Mouse) Press(button MouseButton) error {
	if button < 0 || int(button) >= len(pressCommands) {
		return NewCommandError(fmt.Sprintf("unsupported button: %v", button))
	}
	_, err := m.transport.SendCommand(pressCommands[button], false, 0)
	return err
}

// Release sends a button-release command.
func (m *Mouse) Release(button MouseButton) error {
	if button < 0 || int(button) >= len(releaseCommands) {
		return NewCommandError(fmt.Sprintf("unsupported button: %v", button))
	}
	_, err := m.transport.SendCommand(releaseCommands[button], false, 0)
	return err
}

// Click presses and immediately releases a button.
func (m *Mouse) Click(button MouseButton) error {
	if err := m.Press(button); err != nil {
		return err
	}
	return m.Release(button)
}

// Move sends a relative mouse movement.
func (m *Mouse) Move(x, y int) error {
	_, err := m.transport.SendCommand(fmt.Sprintf("km.move(%d,%d)", x, y), false, 0)
	return err
}

// MoveSmooth sends a segmented smooth relative movement.
func (m *Mouse) MoveSmooth(x, y, segments int) error {
	_, err := m.transport.SendCommand(fmt.Sprintf("km.move(%d,%d,%d)", x, y, segments), false, 0)
	return err
}

// MoveBezier sends a bezier-curve relative movement with a control point.
func (m *Mouse) MoveBezier(x, y, segments, ctrlX, ctrlY int) error {
	_, err := m.transport.SendCommand(
		fmt.Sprintf("km.move(%d,%d,%d,%d,%d)", x, y, segments, ctrlX, ctrlY), false, 0)
	return err
}

// Scroll sends a scroll wheel command (positive = up, negative = down).
func (m *Mouse) Scroll(delta int) error {
	_, err := m.transport.SendCommand(fmt.Sprintf("km.wheel(%d)", delta), false, 0)
	return err
}

// --- lock methods ---

func (m *Mouse) setLock(name string, lock bool) error {
	info, ok := lockTargets[name]
	if !ok {
		return NewCommandError(fmt.Sprintf("unknown lock target: %s", name))
	}

	cmd := info.unlockCmd
	if lock {
		cmd = info.lockCmd
	}

	_, err := m.transport.SendCommand(cmd, false, 0)
	if err != nil {
		return err
	}

	if lock {
		m.lockStatesCache |= 1 << info.bit
	} else {
		m.lockStatesCache &= ^(1 << info.bit)
	}
	m.cacheValid = true
	return nil
}

// LockLeft locks/unlocks the left mouse button.
func (m *Mouse) LockLeft(lock bool) error { return m.setLock("LEFT", lock) }

// LockRight locks/unlocks the right mouse button.
func (m *Mouse) LockRight(lock bool) error { return m.setLock("RIGHT", lock) }

// LockMiddle locks/unlocks the middle mouse button.
func (m *Mouse) LockMiddle(lock bool) error { return m.setLock("MIDDLE", lock) }

// LockSide1 locks/unlocks mouse button 4.
func (m *Mouse) LockSide1(lock bool) error { return m.setLock("MOUSE4", lock) }

// LockSide2 locks/unlocks mouse button 5.
func (m *Mouse) LockSide2(lock bool) error { return m.setLock("MOUSE5", lock) }

// LockX locks/unlocks the X axis.
func (m *Mouse) LockX(lock bool) error { return m.setLock("X", lock) }

// LockY locks/unlocks the Y axis.
func (m *Mouse) LockY(lock bool) error { return m.setLock("Y", lock) }

// IsLocked checks whether the given button is currently locked.
func (m *Mouse) IsLocked(button MouseButton) (bool, error) {
	name := strings.ToUpper(button.String())
	if name == "MOUSE4" || name == "MOUSE5" {
		// already correct
	}

	info, ok := lockTargets[name]
	if !ok {
		return false, NewCommandError(fmt.Sprintf("unsupported lock target: %v", button))
	}

	if m.cacheValid {
		return m.lockStatesCache&(1<<info.bit) != 0, nil
	}

	resp, err := m.transport.SendCommand(info.queryCmd, true, 50*time.Millisecond)
	if err != nil {
		return false, err
	}

	locked := strings.TrimSpace(resp) == "1"
	if locked {
		m.lockStatesCache |= 1 << info.bit
	} else {
		m.lockStatesCache &= ^(1 << info.bit)
	}
	return locked, nil
}

// GetAllLockStates returns the lock state of every button and axis.
func (m *Mouse) GetAllLockStates() (map[string]bool, error) {
	if m.cacheValid {
		return map[string]bool{
			"LEFT":   m.lockStatesCache&(1<<0) != 0,
			"RIGHT":  m.lockStatesCache&(1<<1) != 0,
			"MIDDLE": m.lockStatesCache&(1<<2) != 0,
			"MOUSE4": m.lockStatesCache&(1<<3) != 0,
			"MOUSE5": m.lockStatesCache&(1<<4) != 0,
			"X":      m.lockStatesCache&(1<<5) != 0,
			"Y":      m.lockStatesCache&(1<<6) != 0,
		}, nil
	}

	states := make(map[string]bool, 7)
	targets := []string{"LEFT", "RIGHT", "MIDDLE", "MOUSE4", "MOUSE5", "X", "Y"}

	for _, name := range targets {
		info := lockTargets[name]
		resp, err := m.transport.SendCommand(info.queryCmd, true, 50*time.Millisecond)
		if err != nil {
			states[name] = false
			continue
		}
		locked := strings.TrimSpace(resp) == "1"
		states[name] = locked
		if locked {
			m.lockStatesCache |= 1 << info.bit
		} else {
			m.lockStatesCache &= ^(1 << info.bit)
		}
	}

	m.cacheValid = true
	return states, nil
}

// SpoofSerial sets a custom serial number on the device.
func (m *Mouse) SpoofSerial(serial string) error {
	_, err := m.transport.SendCommand(fmt.Sprintf("km.serial('%s')", serial), false, 0)
	return err
}

// ResetSerial resets the device serial number to factory default.
func (m *Mouse) ResetSerial() error {
	_, err := m.transport.SendCommand("km.serial(0)", false, 0)
	return err
}

// GetDeviceInfo returns information about the connected device and its COM port.
func (m *Mouse) GetDeviceInfo() DeviceInfo {
	port := m.transport.Port
	connected := m.transport.IsConnected()

	if !connected || port == "" {
		return DeviceInfo{
			Port:        port,
			Description: "Disconnected",
			VID:         "Unknown",
			PID:         "Unknown",
			IsConnected: false,
		}
	}

	info := DeviceInfo{
		Port:        port,
		Description: "Connected Device",
		VID:         "Unknown",
		PID:         "Unknown",
		IsConnected: true,
	}

	ports, err := enumerator.GetDetailedPortsList()
	if err == nil {
		for _, p := range ports {
			if p.Name == port {
				if p.Product != "" {
					info.Description = p.Product
				}
				if p.IsUSB {
					info.VID = p.VID
					info.PID = p.PID
				}
				break
			}
		}
	}

	return info
}

// GetFirmwareVersion queries the device for its firmware version string.
func (m *Mouse) GetFirmwareVersion() (string, error) {
	resp, err := m.transport.SendCommand("km.version()", true, 100*time.Millisecond)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// InvalidateCache marks the lock-state cache as stale.
func (m *Mouse) InvalidateCache() {
	m.cacheValid = false
}
