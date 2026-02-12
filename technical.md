# Technical Guide: Converting a Python Library to Go

This document outlines the approach, decisions, and patterns used when porting the **Makcu Python library** (`makcu-py-lib`) to Go (`Makcu-go-lib`). It serves as a general reference for converting Python packages into idiomatic Go.

---

## 1. Structural Mapping

### Python → Go Package Layout

| Python                        | Go                          | Notes                                                  |
|-------------------------------|-----------------------------|--------------------------------------------------------|
| `__init__.py`                 | `macku.go`                  | Package-level doc comment and version constant         |
| `enums.py` (Enum classes)     | `enums.go` (typed consts)   | Use `iota` or explicit `int`/`string` typed constants  |
| `errors.py` (Exception classes) | `errors.go` (sentinel errors + struct) | Go uses `errors.Is()` / `errors.As()` instead of `except` |
| `connection.py` (class)       | `connection.go` (struct)    | Class → struct, methods → receiver functions           |
| `mouse.py` (class)            | `mouse.go` (struct)         | Same pattern                                           |
| `controller.py` (class)       | `controller.go` (struct)    | Same pattern                                           |
| Platform-specific code        | Build-tagged files           | `//go:build windows` and `//go:build !windows`         |
| `requirements.txt`            | `go.mod` / `go.sum`         | Dependencies managed by Go modules                     |
| `test_suite.py` (pytest)      | `tests/lib_test.go`         | `testing` package with `go test`                       |

### Key Principle

Python packages are directories with `__init__.py`. Go packages are directories where **all `.go` files share the same `package` declaration**. There is no `__init__.go` — exports are controlled by capitalisation (uppercase = public).

---

## 2. Type System Conversion

### Enums

Python uses `enum.Enum` classes. Go has no built-in enums, so the idiomatic approach is:

```python
# Python
class MouseButton(Enum):
    LEFT = 0
    RIGHT = 1
    MIDDLE = 2
```

```go
// Go
type MouseButton int

const (
    MouseButtonLeft   MouseButton = 0
    MouseButtonRight  MouseButton = 1
    MouseButtonMiddle MouseButton = 2
)
```

Add a `String()` method to satisfy `fmt.Stringer` for readable output.

### Classes → Structs

Every Python class becomes a Go struct. The constructor (`__init__`) becomes a `New...` function:

```python
# Python
class Mouse:
    def __init__(self, transport):
        self.transport = transport
```

```go
// Go
type Mouse struct {
    transport *SerialTransport
}

func NewMouse(transport *SerialTransport) *Mouse {
    return &Mouse{transport: transport}
}
```

### Visibility

- Python: convention-based (`_private`, `public`)
- Go: **capitalisation-based** — `Exported` (uppercase) vs `unexported` (lowercase)

Map Python's `__all__` exports to uppercase Go identifiers. Internal helpers become lowercase.

---

## 3. Error Handling

This is the single largest conceptual shift.

### Python: Exceptions

```python
class MakcuConnectionError(MakcuError):
    pass

try:
    makcu.connect()
except MakcuConnectionError as e:
    print(f"Failed: {e}")
```

### Go: Return Values

```go
var ErrConnection = errors.New("macku: connection error")

type MakcuError struct {
    Base    error
    Message string
}

func (e *MakcuError) Unwrap() error { return e.Base }

// Usage
err := controller.Connect()
if errors.Is(err, ErrConnection) {
    fmt.Println("Failed:", err)
}
```

### Rules

1. **Every function that can fail must return `error`** as the last return value
2. Replace `raise` with `return NewXxxError("message")`
3. Replace `try/except` with `if err != nil { ... }`
4. Use **sentinel errors** (`var ErrXxx = errors.New(...)`) with `errors.Is()` for type-based catching
5. Use **wrapped errors** (`fmt.Errorf("...: %w", err)`) to preserve error chains

---

## 4. Concurrency: Threading → Goroutines

### Python Threads

```python
self._listener_thread = threading.Thread(target=self._listen, daemon=True)
self._listener_thread.start()
self._stop_event = threading.Event()
```

### Go Goroutines

```go
s.stopChan = make(chan struct{})
go s.listen()

// To stop:
close(s.stopChan)
```

| Python                     | Go                                    |
|----------------------------|---------------------------------------|
| `threading.Thread`         | `go func()`                           |
| `threading.Event`          | `chan struct{}`                        |
| `threading.Lock`           | `sync.Mutex`                          |
| `concurrent.futures.Future`| `chan T` (buffered channel)            |
| `asyncio` / `await`        | Goroutines + channels (not needed)    |
| `ThreadPoolExecutor`       | Not needed — goroutines are cheap     |

### The `maybe_async` Pattern

The Python library uses a `@maybe_async` decorator so every method works both synchronously and asynchronously. **This is entirely unnecessary in Go.** All Go functions are synchronous by default, and the caller can trivially wrap any call in `go func() { ... }()` to run it concurrently. Remove this pattern entirely.

### Future → Channel

Python uses `concurrent.futures.Future` for waiting on command responses. In Go, use a buffered channel:

```python
# Python
future = Future()
result = future.result(timeout=0.1)
```

```go
// Go
resultCh := make(chan string, 1)

select {
case result := <-resultCh:
    // success
case <-time.After(timeout):
    // timeout
}
```

---

## 5. Serial Communication

The Python library uses `pyserial`. The Go equivalent is `go.bug.st/serial`, which provides cross-platform serial port access with a similar API:

| pyserial                     | go.bug.st/serial                  |
|------------------------------|-----------------------------------|
| `serial.Serial(port, baud)`  | `serial.Open(port, &serial.Mode{BaudRate: baud})` |
| `ser.write(data)`            | `port.Write(data)`                |
| `ser.read(n)`                | `port.Read(buf)`                  |
| `ser.in_waiting`             | Read returns 0 bytes (non-blocking with timeout) |
| `ser.close()`                | `port.Close()`                    |
| `serial.tools.list_ports`    | `serial/enumerator` subpackage    |

### Baud Rate Changes

Both libraries support changing the baud rate after opening. In Go:

```go
port.SetMode(&serial.Mode{BaudRate: 4000000})
```

---

## 6. Platform-Specific Code

### Python

Python uses runtime checks:
```python
import ctypes
ctypes.windll.user32.GetCursorPos(...)  # Only works on Windows
```

### Go — Build Tags

Go uses **compile-time file selection** via build tags:

```go
// mouse_windows.go
//go:build windows
package Macku
// Uses syscall to call user32.dll

// mouse_stub.go
//go:build !windows
package Macku
func (m *Mouse) MoveAbs(...) error {
    return errors.New("MoveAbs is only supported on Windows")
}
```

This is cleaner than runtime checks — the unsupported code is never compiled.

### Windows API Calls

Python uses `ctypes` to call Windows APIs. Go uses `syscall` / `golang.org/x/sys/windows`:

```python
# Python
ctypes.windll.user32.GetCursorPos(ctypes.byref(pt))
```

```go
// Go
var user32 = syscall.NewLazyDLL("user32.dll")
var procGetCursorPos = user32.NewProc("GetCursorPos")
procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
```

---

## 7. Testing

### Python (pytest)

```python
def test_click(makcu):
    makcu.click(MouseButton.LEFT)
```

### Go (testing package)

```go
func TestClick(t *testing.T) {
    // ...
    if err != nil {
        t.Errorf("Click failed: %v", err)
    }
}
```

| pytest                       | Go testing                            |
|------------------------------|---------------------------------------|
| `def test_xxx():`            | `func TestXxx(t *testing.T)`          |
| `assert x == y`             | `if x != y { t.Errorf(...) }`        |
| `@pytest.fixture`           | `TestMain` or helper functions        |
| `pytest.raises(XxxError)`   | `if !errors.Is(err, ErrXxx)`         |
| `conftest.py`               | Shared helpers in `_test.go` files    |

### Hardware-Free Testing

Since the Makcu device may not be available in CI, tests should focus on:
- Enum/constant correctness
- Error type behaviour
- Config defaults
- Constructor validity
- Disconnected-state error paths

---

## 8. Common Pitfalls

### 1. Don't Translate Async/Await

Go doesn't have `async`/`await`. Every function runs synchronously. Callers use goroutines for concurrency. Remove all `async def`, `await`, `asyncio` patterns — replace with normal functions.

### 2. Don't Use `interface{}` as a Catch-All

Python's dynamic typing means functions accept `Union[MouseButton, str]`. In Go, use separate methods or typed parameters rather than `interface{}`:

```python
# Python
def lock(self, target: Union[MouseButton, str]): ...
```

```go
// Go — use separate typed approach
func (c *MakcuController) Lock(target LockTarget) error { ... }
// LockTarget is a concrete type, not a union
```

### 3. Close Resources Explicitly

Python has garbage collection and `__del__`. Go requires explicit `Close()`/`Disconnect()` calls. Use `defer`:

```go
controller, _ := Macku.CreateController(cfg)
defer controller.Disconnect()
```

### 4. Don't Use `time.Sleep` for Synchronisation

Where Python uses `time.sleep()` to wait for threads, Go should use channels, `sync.WaitGroup`, or `select` statements.

### 5. Capitalisation Matters

Every identifier that needs to be accessible from outside the package **must** start with an uppercase letter. This is the most common mistake when porting from Python.

---

## 9. Dependency Management

| Python                    | Go                                |
|---------------------------|-----------------------------------|
| `requirements.txt`        | `go.mod`                          |
| `pip install`             | `go get`                          |
| `setup.py` / `pyproject.toml` | `go.mod` (module path = import path) |
| PyPI                      | Go module proxy (`proxy.golang.org`) |
| Virtual environments      | Not needed — modules are per-project |

---

## 10. Summary Checklist

When converting a Python library to Go:

- [ ] Map the directory structure (one Go package per directory)
- [ ] Convert Enum classes to typed constants
- [ ] Convert Exception hierarchy to sentinel errors + wrapping
- [ ] Convert classes to structs with constructor functions
- [ ] Replace `threading.Thread` with goroutines
- [ ] Replace `threading.Lock` with `sync.Mutex`
- [ ] Replace `Future` / callback patterns with channels
- [ ] Remove all `async`/`await` — Go doesn't need it
- [ ] Use build tags for platform-specific code
- [ ] Replace `ctypes` Windows calls with `syscall`
- [ ] Replace `pyserial` with `go.bug.st/serial`
- [ ] Make every fallible function return `error`
- [ ] Uppercase all public identifiers
- [ ] Write tests using the `testing` package
- [ ] Run `go vet` and `go build` for both Windows and Linux targets
