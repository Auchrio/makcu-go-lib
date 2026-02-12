# 🖱️ Makcu Go Library v2.3.0

[![Go Reference](https://pkg.go.dev/badge/github.com/Auchrio/Makcu-go-lib.svg)](https://pkg.go.dev/github.com/Auchrio/Makcu-go-lib)
[![License](https://img.shields.io/badge/license-GPL-blue.svg)](LICENSE)

Makcu Go Lib is a high-performance Go library for controlling Makcu devices — featuring **zero-delay command execution**, **goroutine-based listener**, and **automatic reconnection**. A Go port of [SleepyTotem/makcu-py-lib](https://github.com/SleepyTotem/makcu-py-lib).

---

## 📦 Installation

```bash
go get github.com/Auchrio/Makcu-go-lib
```

### From Source

```bash
git clone https://github.com/Auchrio/Makcu-go-lib
cd Makcu-go-lib
go build ./...
```

---

## 🧠 Quick Start

```go
package main

import (
    "fmt"
    "log"

    Macku "github.com/Auchrio/Makcu-go-lib"
)

func main() {
    cfg := Macku.DefaultConfig()
    cfg.Debug = true

    controller, err := Macku.CreateController(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer controller.Disconnect()

    // Basic operations
    controller.Click(Macku.MouseButtonLeft)
    controller.Move(100, 50)
    controller.Scroll(-1)

    // Human-like interaction
    controller.ClickHumanLike(Macku.MouseButtonLeft, 2, Macku.ProfileGaming, 3)

    fmt.Println("Done!")
}
```

---

## 🎮 Core Features

### Mouse Control

```go
// Button actions
controller.Click(Macku.MouseButtonLeft)
controller.DoubleClick(Macku.MouseButtonRight)
controller.Press(Macku.MouseButtonMiddle)
controller.Release(Macku.MouseButtonMiddle)

// Movement
controller.Move(100, 50)                // Relative movement
controller.MoveSmooth(200, 100, 20)     // Smooth interpolation
controller.MoveBezier(150, 150, 30, &cx, &cy) // Bezier curve (pass nil for defaults)

// Absolute movement (Windows only)
controller.MoveAbs([2]int{500, 300}, 1, 2)

// Scrolling
controller.Scroll(-5) // Scroll down
controller.Scroll(3)  // Scroll up

// Dragging
controller.Drag(0, 0, 300, 200, Macku.MouseButtonLeft, 1500*time.Millisecond)
```

### Button & Axis Locking

```go
// Unified locking API
controller.Lock(Macku.LockLeft)    // Lock left button
controller.Unlock(Macku.LockRight) // Unlock right button
controller.Lock(Macku.LockX)       // Lock X-axis movement
controller.Unlock(Macku.LockY)     // Unlock Y-axis movement

// Direct lock methods
controller.LockLeft(true)
controller.LockRight(false)
controller.LockX(true)

// Query lock states (cached — no serial delay!)
locked, _ := controller.IsLocked(Macku.MouseButtonLeft)
allStates, _ := controller.GetAllLockStates()
// Returns: map["LEFT":true "RIGHT":false "X":true ...]
```

### Human-like Interactions

```go
// Realistic clicking with timing variations
controller.ClickHumanLike(
    Macku.MouseButtonLeft,
    5,                    // count
    Macku.ProfileGaming,  // "normal", "fast", "slow", "variable", "gaming"
    5,                    // jitter — random pixel movement between clicks
)
```

### Button Event Monitoring

```go
// Real-time button monitoring
controller.SetButtonCallback(func(button Macku.MouseButton, pressed bool) {
    action := "released"
    if pressed {
        action = "pressed"
    }
    fmt.Printf("%s %s\n", button, action)
})
controller.EnableButtonMonitoring(true)

// Check current button states
states, _ := controller.GetButtonStates()
pressed, _ := controller.IsPressed(Macku.MouseButtonRight)
if pressed {
    fmt.Println("Right button is pressed")
}
```

### Connection Management

```go
// Auto-reconnection on disconnect
cfg := Macku.DefaultConfig()
cfg.AutoReconnect = true
controller, _ := Macku.CreateController(cfg)

// Connection status callbacks
controller.OnConnectionChange(func(connected bool) {
    if connected {
        fmt.Println("Device reconnected!")
    } else {
        fmt.Println("Device disconnected!")
    }
})

// Manual reconnection
if !controller.IsConnected() {
    controller.Connect()
}
```

---

## 🔧 Advanced Features

### Batch Operations

```go
// Execute multiple commands efficiently
controller.BatchExecute([]func() error{
    func() error { return controller.Move(50, 0) },
    func() error { return controller.Click(Macku.MouseButtonLeft) },
    func() error { return controller.Move(-50, 0) },
    func() error { return controller.Click(Macku.MouseButtonRight) },
})
```

### Device Information

```go
// Get device details
info, _ := controller.GetDeviceInfo()
// DeviceInfo{Port:"COM3", VID:"1A86", PID:"55D3", IsConnected:true, ...}

// Firmware version
version, _ := controller.GetFirmwareVersion()
```

### Serial Spoofing

```go
// Spoof device serial
controller.SpoofSerial("CUSTOM123456")

// Reset to default
controller.ResetSerial()
```

### Low-Level Access

```go
// Send raw commands with tracked responses
response, err := controller.Transport.SendCommand(
    "km.version()",
    true,                    // expect response
    100*time.Millisecond,    // timeout
)
```

---

## 🧪 Running Tests

```bash
go test -v ./tests
```

Tests cover enums, errors, config, controller construction, and disconnected-error handling — no hardware required.

---

## 🏎️ Performance Optimization Details

### Key Optimizations

1. **Pre-computed Commands**: All press/release commands are pre-formatted at init
2. **Bitwise Operations**: Button states use single integer with bit manipulation
3. **Goroutine Listener**: Dedicated goroutine for serial reads with channel-based response routing
4. **Zero-Copy Buffers**: Pre-allocated buffers for parsing
5. **Reduced Timeouts**: Gaming-optimized timeouts (100ms default)
6. **Cache Everything**: Lock states and device info cached with invalidation
7. **Minimal Allocations**: Reuse buffers, avoid string formatting in hot paths
8. **Fast Serial Settings**: 1ms read timeout, 4Mbps baud rate

### Tips for Maximum Performance

```go
// Disable debug mode in production
cfg := Macku.DefaultConfig()
cfg.Debug = false
controller, _ := Macku.CreateController(cfg)

// Use cached connection checks
if controller.IsConnected() { // Cached, no serial check
    controller.Click(Macku.MouseButtonLeft)
}

// Batch similar operations
for i := 0; i < 10; i++ {
    controller.Move(10, 0)
}
```

---

## 🔍 Debugging

Enable debug mode for detailed logging:

```go
cfg := Macku.DefaultConfig()
cfg.Debug = true
controller, _ := Macku.CreateController(cfg)

// View command flow (timestamped)
// [12:34:56] [INFO] Command 'km.move(100,50)' sent (no response expected)
// [12:34:56] [INFO] Command 'km.version()' completed
```

---

## 📚 API Reference

### Enumerations

```go
import Macku "github.com/Auchrio/Makcu-go-lib"

Macku.MouseButtonLeft   // Left mouse button
Macku.MouseButtonRight  // Right mouse button
Macku.MouseButtonMiddle // Middle mouse button
Macku.MouseButton4      // Side button 1
Macku.MouseButton5      // Side button 2
```

### Error Handling

```go
import "errors"

controller, err := Macku.CreateController(cfg)
if err != nil {
    if errors.Is(err, Macku.ErrConnection) {
        fmt.Println("Connection failed:", err)
    } else if errors.Is(err, Macku.ErrTimeout) {
        fmt.Println("Command timed out:", err)
    }
}
```

### Platform Support

| Feature | Windows | Linux | macOS |
|---------|---------|-------|-------|
| All core features | ✅ | ✅ | ✅ |
| `MoveAbs` | ✅ | ❌ | ❌ |

`MoveAbs` uses Windows `GetCursorPos`/`SystemParametersInfoW` APIs. On other platforms it returns an error.

---

## 🛠️ Technical Details

- **Protocol**: CH343 USB serial at 4Mbps
- **Command Format**: ASCII with optional ID tracking (`command#ID`)
- **Response Parsing**: Goroutine listener with text/button-data disambiguation
- **Auto-Discovery**: VID:PID=1A86:55D3 detection via `go.bug.st/serial`
- **Buffer Size**: 4KB read buffer, 256B line buffer
- **Cleanup Interval**: 50ms for timed-out commands
- **Dependencies**: `go.bug.st/serial` (cross-platform serial I/O)

---

## 📜 License

GPL License © SleepyTotem / Auchrio

---

## 🙋 Support

- **Issues**: [GitHub Issues](https://github.com/Auchrio/Makcu-go-lib/issues)
- **Original Python Library**: [SleepyTotem/makcu-py-lib](https://github.com/SleepyTotem/makcu-py-lib)

---

## 🌐 Links

- [GitHub Repository](https://github.com/Auchrio/Makcu-go-lib)
- [Go Package Docs](https://pkg.go.dev/github.com/Auchrio/Makcu-go-lib)
- [Original Python Library](https://github.com/SleepyTotem/makcu-py-lib)
