package lib_test

import (
	"errors"
	"testing"

	Macku "github.com/Auchrio/Makcu-go-lib"
)

// ---------------------------------------------------------------------------
// Enum tests
// ---------------------------------------------------------------------------

func TestMouseButtonValues(t *testing.T) {
	tests := []struct {
		button Macku.MouseButton
		want   int
	}{
		{Macku.MouseButtonLeft, 0},
		{Macku.MouseButtonRight, 1},
		{Macku.MouseButtonMiddle, 2},
		{Macku.MouseButton4, 3},
		{Macku.MouseButton5, 4},
	}
	for _, tt := range tests {
		if int(tt.button) != tt.want {
			t.Errorf("MouseButton %v = %d, want %d", tt.button, int(tt.button), tt.want)
		}
	}
}

func TestMouseButtonString(t *testing.T) {
	tests := []struct {
		button Macku.MouseButton
		want   string
	}{
		{Macku.MouseButtonLeft, "left"},
		{Macku.MouseButtonRight, "right"},
		{Macku.MouseButtonMiddle, "middle"},
		{Macku.MouseButton4, "mouse4"},
		{Macku.MouseButton5, "mouse5"},
		{Macku.MouseButton(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.button.String(); got != tt.want {
			t.Errorf("MouseButton(%d).String() = %q, want %q", tt.button, got, tt.want)
		}
	}
}

func TestLockTargetValues(t *testing.T) {
	targets := []Macku.LockTarget{
		Macku.LockLeft, Macku.LockRight, Macku.LockMiddle,
		Macku.LockMouse4, Macku.LockMouse5, Macku.LockX, Macku.LockY,
	}
	seen := make(map[Macku.LockTarget]bool, len(targets))
	for _, lt := range targets {
		if seen[lt] {
			t.Errorf("Duplicate LockTarget value: %d", lt)
		}
		seen[lt] = true
	}
}

func TestClickProfileConstants(t *testing.T) {
	profiles := []Macku.ClickProfile{
		Macku.ProfileNormal, Macku.ProfileFast, Macku.ProfileSlow,
		Macku.ProfileVariable, Macku.ProfileGaming,
	}
	for _, p := range profiles {
		if p == "" {
			t.Error("ClickProfile should not be empty")
		}
	}
}

// ---------------------------------------------------------------------------
// Error tests
// ---------------------------------------------------------------------------

func TestConnectionError(t *testing.T) {
	err := Macku.NewConnectionError("test connection failure")
	if !errors.Is(err, Macku.ErrConnection) {
		t.Error("NewConnectionError should wrap ErrConnection")
	}
	if err.Error() != "test connection failure" {
		t.Errorf("Error message = %q, want %q", err.Error(), "test connection failure")
	}
}

func TestCommandError(t *testing.T) {
	err := Macku.NewCommandError("bad command")
	if !errors.Is(err, Macku.ErrCommand) {
		t.Error("NewCommandError should wrap ErrCommand")
	}
}

func TestTimeoutError(t *testing.T) {
	err := Macku.NewTimeoutError("timed out")
	if !errors.Is(err, Macku.ErrTimeout) {
		t.Error("NewTimeoutError should wrap ErrTimeout")
	}
}

func TestResponseError(t *testing.T) {
	err := Macku.NewResponseError("bad response")
	if !errors.Is(err, Macku.ErrResponse) {
		t.Error("NewResponseError should wrap ErrResponse")
	}
}

// ---------------------------------------------------------------------------
// Config tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := Macku.DefaultConfig()

	if !cfg.SendInit {
		t.Error("DefaultConfig.SendInit should be true")
	}
	if !cfg.AutoReconnect {
		t.Error("DefaultConfig.AutoReconnect should be true")
	}
	if cfg.Debug {
		t.Error("DefaultConfig.Debug should be false")
	}
	if cfg.OverridePort {
		t.Error("DefaultConfig.OverridePort should be false")
	}
	if cfg.FallbackCOMPort != "" {
		t.Errorf("DefaultConfig.FallbackCOMPort should be empty, got %q", cfg.FallbackCOMPort)
	}
}

// ---------------------------------------------------------------------------
// Controller construction tests (no hardware required)
// ---------------------------------------------------------------------------

func TestNewController(t *testing.T) {
	cfg := Macku.DefaultConfig()
	c := Macku.NewController(cfg)

	if c == nil {
		t.Fatal("NewController returned nil")
	}
	if c.Transport == nil {
		t.Error("Controller.Transport should not be nil")
	}
	if c.Mouse == nil {
		t.Error("Controller.Mouse should not be nil")
	}
	if c.IsConnected() {
		t.Error("New controller should not be connected")
	}
}

func TestNewControllerWithOptions(t *testing.T) {
	cfg := Macku.Config{
		FallbackCOMPort: "COM7",
		Debug:           true,
		SendInit:        false,
		AutoReconnect:   false,
		OverridePort:    true,
	}
	c := Macku.NewController(cfg)

	if c == nil {
		t.Fatal("NewController returned nil")
	}
	if c.IsConnected() {
		t.Error("New controller should not be connected")
	}
}

// ---------------------------------------------------------------------------
// DeviceInfo struct test
// ---------------------------------------------------------------------------

func TestDeviceInfoStruct(t *testing.T) {
	info := Macku.DeviceInfo{
		Port:        "COM3",
		Description: "Test Device",
		VID:         "1A86",
		PID:         "55D3",
		IsConnected: true,
	}
	if info.Port != "COM3" {
		t.Errorf("DeviceInfo.Port = %q, want %q", info.Port, "COM3")
	}
	if !info.IsConnected {
		t.Error("DeviceInfo.IsConnected should be true")
	}
}

// ---------------------------------------------------------------------------
// Version test
// ---------------------------------------------------------------------------

func TestVersion(t *testing.T) {
	if Macku.Version == "" {
		t.Error("Version should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Disconnected controller error handling
// ---------------------------------------------------------------------------

func TestDisconnectedControllerReturnsError(t *testing.T) {
	c := Macku.NewController(Macku.DefaultConfig())

	if err := c.Click(Macku.MouseButtonLeft); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Click on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Move(10, 10); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Move on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Scroll(1); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Scroll on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Press(Macku.MouseButtonLeft); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Press on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Release(Macku.MouseButtonLeft); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Release on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Lock(Macku.LockLeft); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Lock on disconnected controller: got %v, want connection error", err)
	}
	if err := c.Unlock(Macku.LockX); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("Unlock on disconnected controller: got %v, want connection error", err)
	}
	if _, err := c.GetFirmwareVersion(); !errors.Is(err, Macku.ErrConnection) {
		t.Errorf("GetFirmwareVersion on disconnected controller: got %v, want connection error", err)
	}
}
