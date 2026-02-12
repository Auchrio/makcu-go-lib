package Macku

import (
	"fmt"
	"math/rand"
	"time"
)

// Config holds all options for creating a MakcuController.
type Config struct {
	FallbackCOMPort string // COM port to use when auto-detection fails
	Debug           bool   // Enable verbose debug logging
	SendInit        bool   // Send km.buttons(1) on connect
	AutoReconnect   bool   // Auto-reconnect on serial errors
	OverridePort    bool   // Skip auto-detection and use FallbackCOMPort directly
}

// DefaultConfig returns a Config with sensible defaults (SendInit and AutoReconnect enabled).
func DefaultConfig() Config {
	return Config{
		SendInit:      true,
		AutoReconnect: true,
	}
}

// timingProfile defines min/max durations (ms) for human-like click timing.
type timingProfile struct {
	minDown, maxDown, minWait, maxWait int
}

var clickProfiles = map[ClickProfile]timingProfile{
	ProfileNormal:   {60, 120, 100, 180},
	ProfileFast:     {30, 60, 50, 100},
	ProfileSlow:     {100, 180, 150, 300},
	ProfileVariable: {40, 200, 80, 250},
	ProfileGaming:   {20, 40, 30, 60},
}

// MakcuController is the high-level API for interacting with a Makcu device.
type MakcuController struct {
	Transport *SerialTransport
	Mouse     *Mouse

	connected           bool
	connectionCallbacks []func(bool)
}

// NewController creates (but does not connect) a new MakcuController.
func NewController(cfg Config) *MakcuController {
	transport := NewSerialTransport(
		cfg.FallbackCOMPort,
		cfg.Debug,
		cfg.SendInit,
		cfg.AutoReconnect,
		cfg.OverridePort,
	)
	return &MakcuController{
		Transport: transport,
		Mouse:     NewMouse(transport),
	}
}

// CreateController creates a MakcuController and connects it immediately.
func CreateController(cfg Config) (*MakcuController, error) {
	c := NewController(cfg)
	if err := c.Connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *MakcuController) checkConnection() error {
	if !c.connected {
		return NewConnectionError("not connected")
	}
	return nil
}

func (c *MakcuController) notifyConnectionChange(connected bool) {
	for _, cb := range c.connectionCallbacks {
		cb(connected)
	}
}

// --- connection ---

// Connect opens the serial connection to the Makcu device.
func (c *MakcuController) Connect() error {
	if err := c.Transport.Connect(); err != nil {
		return err
	}
	c.connected = true
	c.notifyConnectionChange(true)
	return nil
}

// Disconnect closes the serial connection.
func (c *MakcuController) Disconnect() error {
	err := c.Transport.Disconnect()
	c.connected = false
	c.notifyConnectionChange(false)
	return err
}

// IsConnected returns true if the controller has an active device connection.
func (c *MakcuController) IsConnected() bool {
	return c.connected && c.Transport.IsConnected()
}

// --- basic mouse actions ---

// Click presses and releases a mouse button.
func (c *MakcuController) Click(button MouseButton) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	if err := c.Mouse.Press(button); err != nil {
		return err
	}
	return c.Mouse.Release(button)
}

// DoubleClick performs two rapid clicks.
func (c *MakcuController) DoubleClick(button MouseButton) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	if err := c.Mouse.Press(button); err != nil {
		return err
	}
	if err := c.Mouse.Release(button); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	if err := c.Mouse.Press(button); err != nil {
		return err
	}
	return c.Mouse.Release(button)
}

// Press presses (holds) a mouse button.
func (c *MakcuController) Press(button MouseButton) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.Press(button)
}

// Release releases a mouse button.
func (c *MakcuController) Release(button MouseButton) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.Release(button)
}

// --- movement ---

// Move sends a relative mouse movement.
func (c *MakcuController) Move(dx, dy int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.Move(dx, dy)
}

// MoveAbs moves the cursor to an absolute screen position (Windows only).
func (c *MakcuController) MoveAbs(target [2]int, speed, waitMs int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.MoveAbs(target, speed, waitMs)
}

// MoveSmooth performs a segmented smooth relative movement.
func (c *MakcuController) MoveSmooth(dx, dy, segments int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.MoveSmooth(dx, dy, segments)
}

// MoveBezier performs a bezier-curve relative movement. If ctrlX/ctrlY are nil,
// they default to dx/2 and dy/2.
func (c *MakcuController) MoveBezier(dx, dy, segments int, ctrlX, ctrlY *int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	cx := dx / 2
	cy := dy / 2
	if ctrlX != nil {
		cx = *ctrlX
	}
	if ctrlY != nil {
		cy = *ctrlY
	}
	return c.Mouse.MoveBezier(dx, dy, segments, cx, cy)
}

// Scroll sends a scroll-wheel command.
func (c *MakcuController) Scroll(delta int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.Scroll(delta)
}

// --- lock methods ---

// Lock locks the given target (button or axis).
func (c *MakcuController) Lock(target LockTarget) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.setLockByTarget(target, true)
}

// Unlock unlocks the given target (button or axis).
func (c *MakcuController) Unlock(target LockTarget) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.setLockByTarget(target, false)
}

func (c *MakcuController) setLockByTarget(target LockTarget, lock bool) error {
	switch target {
	case LockLeft:
		return c.Mouse.LockLeft(lock)
	case LockRight:
		return c.Mouse.LockRight(lock)
	case LockMiddle:
		return c.Mouse.LockMiddle(lock)
	case LockMouse4:
		return c.Mouse.LockSide1(lock)
	case LockMouse5:
		return c.Mouse.LockSide2(lock)
	case LockX:
		return c.Mouse.LockX(lock)
	case LockY:
		return c.Mouse.LockY(lock)
	default:
		return fmt.Errorf("invalid lock target: %d", target)
	}
}

// LockLeft locks/unlocks the left mouse button.
func (c *MakcuController) LockLeft(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockLeft(lock)
}

// LockRight locks/unlocks the right mouse button.
func (c *MakcuController) LockRight(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockRight(lock)
}

// LockMiddle locks/unlocks the middle mouse button.
func (c *MakcuController) LockMiddle(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockMiddle(lock)
}

// LockSide1 locks/unlocks mouse button 4.
func (c *MakcuController) LockSide1(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockSide1(lock)
}

// LockSide2 locks/unlocks mouse button 5.
func (c *MakcuController) LockSide2(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockSide2(lock)
}

// LockX locks/unlocks the X axis.
func (c *MakcuController) LockX(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockX(lock)
}

// LockY locks/unlocks the Y axis.
func (c *MakcuController) LockY(lock bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.LockY(lock)
}

// IsLocked checks whether the given button is currently locked.
func (c *MakcuController) IsLocked(button MouseButton) (bool, error) {
	if err := c.checkConnection(); err != nil {
		return false, err
	}
	return c.Mouse.IsLocked(button)
}

// GetAllLockStates returns the lock state for every button and axis.
func (c *MakcuController) GetAllLockStates() (map[string]bool, error) {
	if err := c.checkConnection(); err != nil {
		return nil, err
	}
	return c.Mouse.GetAllLockStates()
}

// --- serial spoofing ---

// SpoofSerial sets a custom serial number on the device.
func (c *MakcuController) SpoofSerial(serial string) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.SpoofSerial(serial)
}

// ResetSerial resets the device serial number to its factory default.
func (c *MakcuController) ResetSerial() error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Mouse.ResetSerial()
}

// --- device info ---

// GetDeviceInfo returns information about the connected device.
func (c *MakcuController) GetDeviceInfo() (DeviceInfo, error) {
	if err := c.checkConnection(); err != nil {
		return DeviceInfo{}, err
	}
	return c.Mouse.GetDeviceInfo(), nil
}

// GetFirmwareVersion queries the device for its firmware version string.
func (c *MakcuController) GetFirmwareVersion() (string, error) {
	if err := c.checkConnection(); err != nil {
		return "", err
	}
	return c.Mouse.GetFirmwareVersion()
}

// --- button monitoring ---

// GetButtonMask returns the raw button bitmask.
func (c *MakcuController) GetButtonMask() (int, error) {
	if err := c.checkConnection(); err != nil {
		return 0, err
	}
	return c.Transport.GetButtonMask(), nil
}

// GetButtonStates returns a map of button name to pressed state.
func (c *MakcuController) GetButtonStates() (map[string]bool, error) {
	if err := c.checkConnection(); err != nil {
		return nil, err
	}
	return c.Transport.GetButtonStates(), nil
}

// IsPressed checks whether the given button is currently held down.
func (c *MakcuController) IsPressed(button MouseButton) (bool, error) {
	if err := c.checkConnection(); err != nil {
		return false, err
	}
	states := c.Transport.GetButtonStates()
	return states[button.String()], nil
}

// EnableButtonMonitoring enables or disables button-state monitoring on the device.
func (c *MakcuController) EnableButtonMonitoring(enable bool) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	return c.Transport.EnableButtonMonitoring(enable)
}

// SetButtonCallback sets a callback invoked when a mouse button state changes.
func (c *MakcuController) SetButtonCallback(cb func(MouseButton, bool)) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	c.Transport.SetButtonCallback(cb)
	return nil
}

// --- connection callbacks ---

// OnConnectionChange registers a callback invoked when the connection state changes.
func (c *MakcuController) OnConnectionChange(cb func(bool)) {
	c.connectionCallbacks = append(c.connectionCallbacks, cb)
}

// RemoveConnectionCallback removes a previously registered connection callback.
// Comparison is done by matching the function pointer.
func (c *MakcuController) RemoveConnectionCallback(cb func(bool)) {
	for i, existing := range c.connectionCallbacks {
		// Best-effort comparison via fmt pointer
		if fmt.Sprintf("%p", existing) == fmt.Sprintf("%p", cb) {
			c.connectionCallbacks = append(c.connectionCallbacks[:i], c.connectionCallbacks[i+1:]...)
			return
		}
	}
}

// --- advanced actions ---

// ClickHumanLike performs one or more human-like clicks with randomised timing.
// Supported profiles: "normal", "fast", "slow", "variable", "gaming".
// jitter adds random pixel movement before each click.
func (c *MakcuController) ClickHumanLike(button MouseButton, count int, profile ClickProfile, jitter int) error {
	if err := c.checkConnection(); err != nil {
		return err
	}

	p, ok := clickProfiles[profile]
	if !ok {
		return fmt.Errorf("invalid click profile: %s", profile)
	}

	for i := 0; i < count; i++ {
		if jitter > 0 {
			dx := rand.Intn(2*jitter+1) - jitter
			dy := rand.Intn(2*jitter+1) - jitter
			c.Mouse.Move(dx, dy)
		}

		c.Mouse.Press(button)
		time.Sleep(time.Duration(rand.Intn(p.maxDown-p.minDown)+p.minDown) * time.Millisecond)
		c.Mouse.Release(button)

		if i < count-1 {
			time.Sleep(time.Duration(rand.Intn(p.maxWait-p.minWait)+p.minWait) * time.Millisecond)
		}
	}

	return nil
}

// Drag performs a mouse drag: moves to (startX,startY), holds the button,
// smooth-moves to (endX,endY), then releases.
func (c *MakcuController) Drag(startX, startY, endX, endY int, button MouseButton, duration time.Duration) error {
	if err := c.checkConnection(); err != nil {
		return err
	}

	if err := c.Move(startX, startY); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)

	if err := c.Press(button); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)

	segments := max(10, int(duration.Seconds()*30))
	if err := c.MoveSmooth(endX-startX, endY-startY, segments); err != nil {
		return err
	}

	time.Sleep(20 * time.Millisecond)
	return c.Release(button)
}

// BatchExecute runs a sequence of actions in order. Execution stops on the first error.
func (c *MakcuController) BatchExecute(actions []func() error) error {
	if err := c.checkConnection(); err != nil {
		return err
	}
	for i, action := range actions {
		if err := action(); err != nil {
			return fmt.Errorf("batch execution failed at action %d: %w", i, err)
		}
	}
	return nil
}
