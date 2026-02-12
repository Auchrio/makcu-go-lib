package Macku

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// baudChangeCommand is the magic byte sequence to switch the device to 4M baud.
var baudChangeCommand = []byte{0xDE, 0xAD, 0x05, 0x00, 0xA5, 0x00, 0x09, 0x3D, 0x00}

// Button name/enum lookup tables.
var (
	buttonNames   = [5]string{"left", "right", "middle", "mouse4", "mouse5"}
	buttonEnumMap = [5]MouseButton{
		MouseButtonLeft, MouseButtonRight, MouseButtonMiddle, MouseButton4, MouseButton5,
	}
)

const (
	DefaultTimeout       = 100 * time.Millisecond
	maxReconnectAttempts = 3
	reconnectDelay       = 100 * time.Millisecond
)

// PendingCommand tracks a command awaiting a response from the device.
type PendingCommand struct {
	CommandID int
	Command   string
	ResultCh  chan string
	Timestamp time.Time
}

// SerialTransport manages the serial connection to a Makcu device.
type SerialTransport struct {
	Port string // The COM port in use (exported for Mouse.GetDeviceInfo)

	fallbackPort  string
	debug         bool
	sendInit      bool
	autoReconnect bool
	overridePort  bool

	isConnected       atomic.Bool
	reconnectAttempts int
	baudrate          int
	serialPort        serial.Port
	currentBaud       int

	commandCounter  int
	pendingCommands map[int]*PendingCommand
	commandLock     sync.Mutex

	buttonCallback func(MouseButton, bool)
	lastButtonMask int
	buttonStates   int

	stopChan chan struct{}
}

// NewSerialTransport creates a new serial transport.
func NewSerialTransport(fallback string, debug, sendInit, autoReconnect, overridePort bool) *SerialTransport {
	s := &SerialTransport{
		fallbackPort:    fallback,
		debug:           debug,
		sendInit:        sendInit,
		autoReconnect:   autoReconnect,
		overridePort:    overridePort,
		baudrate:        115200,
		pendingCommands: make(map[int]*PendingCommand),
		stopChan:        make(chan struct{}),
	}
	s.log("Macku version: %s", Version)
	s.log("Initializing SerialTransport: fallback=%q, debug=%v, sendInit=%v, autoReconnect=%v, overridePort=%v",
		fallback, debug, sendInit, autoReconnect, overridePort)
	return s
}

// log prints a debug message if debug mode is enabled.
func (s *SerialTransport) log(format string, args ...interface{}) {
	if !s.debug {
		return
	}
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] [INFO] %s\n", timestamp, msg)
}

// generateCommandID returns a monotonically increasing command ID (wraps at 10000).
func (s *SerialTransport) generateCommandID() int {
	s.commandCounter = (s.commandCounter + 1) % 10000
	return s.commandCounter
}

// FindCOMPort discovers the Makcu device COM port by USB VID:PID (1A86:55D3),
// falling back to the configured fallback port.
func (s *SerialTransport) FindCOMPort() (string, error) {
	s.log("Starting COM port discovery")

	if s.overridePort {
		s.log("Override port enabled, using: %s", s.fallbackPort)
		return s.fallbackPort, nil
	}

	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		s.log("Error listing ports: %v", err)
		if s.fallbackPort != "" {
			return s.fallbackPort, nil
		}
		return "", fmt.Errorf("failed to list COM ports: %w", err)
	}

	s.log("Found %d COM ports total", len(ports))

	for _, port := range ports {
		s.log("Port: %s - VID: %s PID: %s USB: %v", port.Name, port.VID, port.PID, port.IsUSB)
		if port.IsUSB && strings.ToUpper(port.VID) == "1A86" && strings.ToUpper(port.PID) == "55D3" {
			s.log("Target device found on port: %s", port.Name)
			return port.Name, nil
		}
	}

	s.log("Target device not found in COM port scan")

	if s.fallbackPort != "" {
		s.log("Using fallback COM port: %s", s.fallbackPort)
		return s.fallbackPort, nil
	}

	return "", nil
}

// Connect opens the serial connection, switches to 4M baud, and starts the
// background listener goroutine.
func (s *SerialTransport) Connect() error {
	s.log("Starting connection process")

	if s.isConnected.Load() {
		s.log("Already connected")
		return nil
	}

	if s.overridePort {
		s.Port = s.fallbackPort
	} else {
		port, err := s.FindCOMPort()
		if err != nil {
			return err
		}
		if port == "" {
			return NewConnectionError("Makcu device not found")
		}
		s.Port = port
	}

	s.log("Connecting to %s", s.Port)

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	sp, err := serial.Open(s.Port, mode)
	if err != nil {
		return NewConnectionError(fmt.Sprintf("failed to open %s: %v", s.Port, err))
	}

	s.serialPort = sp

	if err := s.changeBaudTo4M(); err != nil {
		s.serialPort.Close()
		s.serialPort = nil
		return NewConnectionError(fmt.Sprintf("failed to switch to 4M baud: %v", err))
	}

	s.isConnected.Store(true)
	s.reconnectAttempts = 0

	if s.sendInit {
		s.log("Sending initialization command")
		s.serialPort.Write([]byte("km.buttons(1)\r"))
	}

	s.serialPort.SetReadTimeout(time.Millisecond)

	s.stopChan = make(chan struct{})
	go s.listen()

	s.log("Connection established")
	return nil
}

// Disconnect cleanly shuts down the serial connection and listener.
func (s *SerialTransport) Disconnect() error {
	s.log("Starting disconnection process")

	s.isConnected.Store(false)

	// Signal listener goroutine to stop
	select {
	case <-s.stopChan:
		// Already closed
	default:
		close(s.stopChan)
	}

	// Brief wait for listener to exit
	time.Sleep(10 * time.Millisecond)

	// Clear pending commands
	s.commandLock.Lock()
	count := len(s.pendingCommands)
	if count > 0 {
		s.log("Cancelling %d pending commands", count)
	}
	s.pendingCommands = make(map[int]*PendingCommand)
	s.commandLock.Unlock()

	if s.serialPort != nil {
		s.log("Closing serial port: %s", s.Port)
		s.serialPort.Close()
		s.serialPort = nil
	}

	s.log("Disconnection completed")
	return nil
}

// SendCommand sends a command string to the device. If expectResponse is true,
// the call blocks until a response is received or timeout expires.
func (s *SerialTransport) SendCommand(command string, expectResponse bool, timeout time.Duration) (string, error) {
	if !s.isConnected.Load() || s.serialPort == nil {
		return "", NewConnectionError("not connected")
	}

	if timeout == 0 {
		timeout = DefaultTimeout
	}

	if !expectResponse {
		_, err := s.serialPort.Write([]byte(command + "\r\n"))
		if err != nil {
			return "", err
		}
		s.log("Command '%s' sent (no response expected)", command)
		return command, nil
	}

	cmdID := s.generateCommandID()
	resultCh := make(chan string, 1)

	s.commandLock.Lock()
	s.pendingCommands[cmdID] = &PendingCommand{
		CommandID: cmdID,
		Command:   command,
		ResultCh:  resultCh,
		Timestamp: time.Now(),
	}
	s.commandLock.Unlock()

	taggedCmd := fmt.Sprintf("%s#%d\r\n", command, cmdID)
	_, err := s.serialPort.Write([]byte(taggedCmd))
	if err != nil {
		s.commandLock.Lock()
		delete(s.pendingCommands, cmdID)
		s.commandLock.Unlock()
		return "", err
	}

	stopCh := s.stopChan

	select {
	case result := <-resultCh:
		if idx := strings.Index(result, "#"); idx >= 0 {
			result = result[:idx]
		}
		s.log("Command '%s' completed", command)
		return result, nil
	case <-stopCh:
		s.commandLock.Lock()
		delete(s.pendingCommands, cmdID)
		s.commandLock.Unlock()
		return "", NewConnectionError("disconnected while waiting for response")
	case <-time.After(timeout):
		s.commandLock.Lock()
		delete(s.pendingCommands, cmdID)
		s.commandLock.Unlock()
		return "", NewTimeoutError(fmt.Sprintf("command timed out: %s", command))
	}
}

// IsConnected returns true if the transport has an active serial connection.
func (s *SerialTransport) IsConnected() bool {
	return s.isConnected.Load() && s.serialPort != nil
}

// SetButtonCallback sets a function that is called when a mouse button
// state changes. Pass nil to remove the callback.
func (s *SerialTransport) SetButtonCallback(cb func(MouseButton, bool)) {
	s.log("Setting button callback: %v", cb != nil)
	s.buttonCallback = cb
}

// GetButtonStates returns the current pressed state of each mouse button.
func (s *SerialTransport) GetButtonStates() map[string]bool {
	states := make(map[string]bool, 5)
	for i, name := range buttonNames {
		states[name] = s.buttonStates&(1<<i) != 0
	}
	return states
}

// GetButtonMask returns the raw button bitmask from the device.
func (s *SerialTransport) GetButtonMask() int {
	return s.lastButtonMask
}

// EnableButtonMonitoring enables or disables button-state monitoring on the device.
func (s *SerialTransport) EnableButtonMonitoring(enable bool) error {
	cmd := "km.buttons(0)"
	if enable {
		cmd = "km.buttons(1)"
	}
	s.log("%s button monitoring", map[bool]string{true: "Enabling", false: "Disabling"}[enable])
	_, err := s.SendCommand(cmd, false, 0)
	return err
}

// --- internal methods ---

// changeBaudTo4M sends the baud-change magic bytes and switches to 4 000 000 baud.
func (s *SerialTransport) changeBaudTo4M() error {
	s.log("Changing baud rate to 4M")

	if s.serialPort == nil {
		return NewConnectionError("serial port not open")
	}

	_, err := s.serialPort.Write(baudChangeCommand)
	if err != nil {
		return err
	}

	time.Sleep(20 * time.Millisecond)

	err = s.serialPort.SetMode(&serial.Mode{
		BaudRate: 4000000,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	})
	if err != nil {
		return err
	}

	s.currentBaud = 4000000
	s.log("Baud rate changed: 115200 -> 4000000")
	return nil
}

// parseResponseLine extracts the content from a raw response line (strips ">>> " prefix).
func (s *SerialTransport) parseResponseLine(line []byte) string {
	str := strings.TrimSpace(string(line))
	if strings.HasPrefix(str, ">>> ") {
		str = strings.TrimSpace(str[4:])
	}
	return str
}

// handleButtonData processes a raw button-state byte from the device stream.
func (s *SerialTransport) handleButtonData(byteVal int) {
	if byteVal == s.lastButtonMask {
		return
	}

	changedBits := byteVal ^ s.lastButtonMask
	s.log("Button state changed: 0x%02X -> 0x%02X", s.lastButtonMask, byteVal)

	for bit := 0; bit < 8; bit++ {
		if changedBits&(1<<bit) != 0 {
			isPressed := byteVal&(1<<bit) != 0

			if isPressed {
				s.buttonStates |= 1 << bit
			} else {
				s.buttonStates &= ^(1 << bit)
			}

			if s.buttonCallback != nil && bit < len(buttonEnumMap) {
				s.buttonCallback(buttonEnumMap[bit], isPressed)
			}
		}
	}

	s.lastButtonMask = byteVal
}

// processPendingCommands routes a received text response to the oldest pending command.
func (s *SerialTransport) processPendingCommands(content string) {
	if content == "" {
		return
	}

	s.commandLock.Lock()
	defer s.commandLock.Unlock()

	if len(s.pendingCommands) == 0 {
		return
	}

	// Find the oldest pending command (lowest ID).
	oldestID := -1
	for id := range s.pendingCommands {
		if oldestID == -1 || id < oldestID {
			oldestID = id
		}
	}

	pending := s.pendingCommands[oldestID]

	// If the response is just an echo of the command, skip it and wait for the real response.
	if content == pending.Command {
		return
	}

	select {
	case pending.ResultCh <- content:
	default:
	}
	delete(s.pendingCommands, oldestID)
}

// cleanupTimedOutCommands removes stale pending commands that are older than 1 second.
func (s *SerialTransport) cleanupTimedOutCommands() {
	s.commandLock.Lock()
	defer s.commandLock.Unlock()

	now := time.Now()
	for id, pending := range s.pendingCommands {
		if now.Sub(pending.Timestamp) > time.Second {
			s.log("Cleaning up stale command '%s'", pending.Command)
			delete(s.pendingCommands, id)
		}
	}
}

// listen is the background goroutine that reads serial data, parsing text responses
// and button-state bytes. The protocol distinguishes printable text lines (terminated
// by CR+LF) from raw button data (bytes < 32).
func (s *SerialTransport) listen() {
	s.log("Listener goroutine started")

	lineBuffer := make([]byte, 256)
	linePos := 0
	expectingTextMode := false
	lastByte := -1 // -1 = no previous byte

	readBuf := make([]byte, 4096)
	lastCleanup := time.Now()
	cleanupInterval := 50 * time.Millisecond

	for s.isConnected.Load() {
		select {
		case <-s.stopChan:
			s.log("Listener goroutine stopping (stop signal)")
			return
		default:
		}

		n, err := s.serialPort.Read(readBuf)
		if err != nil {
			if s.isConnected.Load() {
				s.log("Serial read error: %v", err)
				if s.autoReconnect {
					s.attemptReconnect()
				} else {
					return
				}
			}
			continue
		}
		if n == 0 {
			continue
		}

		for i := 0; i < n; i++ {
			b := int(readBuf[i])

			switch {
			// Case 1: CR+LF — complete a text line.
			case lastByte == 0x0D && b == 0x0A:
				if linePos > 0 {
					content := s.parseResponseLine(lineBuffer[:linePos])
					linePos = 0
					if content != "" {
						s.processPendingCommands(content)
					}
				}
				expectingTextMode = false

			// Case 2: printable ASCII or TAB — accumulate text.
			case b >= 32 || b == 0x09:
				expectingTextMode = true
				if linePos < 256 {
					lineBuffer[linePos] = byte(b)
					linePos++
				}

			// Case 3: CR — may be start of CRLF.
			case b == 0x0D:
				if expectingTextMode || linePos > 0 {
					expectingTextMode = true
				}

			// Case 4: bare LF — disambiguate between text terminator and button data (0x0A = right+mouse4).
			case b == 0x0A:
				buttonCombo := false

				if s.lastButtonMask != 0 ||
					(lastByte >= 0 && lastByte < 32 && lastByte != 0x0D) ||
					(linePos > 0 && !expectingTextMode) {
					s.handleButtonData(b)
					expectingTextMode = false
					buttonCombo = true
					linePos = 0
				}

				if !buttonCombo {
					if lastByte == 0x0D {
						// Completing CRLF
						if linePos > 0 {
							content := s.parseResponseLine(lineBuffer[:linePos])
							linePos = 0
							if content != "" {
								s.processPendingCommands(content)
							}
						}
						expectingTextMode = false
					} else if linePos > 0 && expectingTextMode {
						// LF-only line end
						content := s.parseResponseLine(lineBuffer[:linePos])
						linePos = 0
						if content != "" {
							s.processPendingCommands(content)
						}
						expectingTextMode = false
					} else if expectingTextMode {
						expectingTextMode = false
					} else {
						s.handleButtonData(b)
						expectingTextMode = false
						linePos = 0
					}
				}

			// Case 5: other control bytes (< 32, excluding TAB/CR/LF) — button data.
			default:
				if lastByte == 0x0D {
					s.handleButtonData(0x0D)
				}
				s.handleButtonData(b)
				expectingTextMode = false
				linePos = 0
			}

			lastByte = b
		}

		// Periodic cleanup of timed-out commands.
		if time.Since(lastCleanup) > cleanupInterval {
			s.cleanupTimedOutCommands()
			lastCleanup = time.Now()
		}
	}

	s.log("Listener goroutine ending")
}

// attemptReconnect tries to re-establish the serial connection after a failure.
func (s *SerialTransport) attemptReconnect() {
	s.log("Attempting reconnect #%d/%d", s.reconnectAttempts+1, maxReconnectAttempts)

	if s.reconnectAttempts >= maxReconnectAttempts {
		s.log("Max reconnect attempts reached, giving up")
		s.isConnected.Store(false)
		return
	}

	s.reconnectAttempts++

	if s.serialPort != nil {
		s.serialPort.Close()
	}

	time.Sleep(reconnectDelay)

	port, err := s.FindCOMPort()
	if err != nil || port == "" {
		s.log("Device not found during reconnect")
		time.Sleep(reconnectDelay)
		return
	}

	s.Port = port

	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	sp, err := serial.Open(s.Port, mode)
	if err != nil {
		s.log("Reconnect open failed: %v", err)
		time.Sleep(reconnectDelay)
		return
	}

	s.serialPort = sp

	if err := s.changeBaudTo4M(); err != nil {
		s.log("Reconnect baud change failed: %v", err)
		s.serialPort.Close()
		time.Sleep(reconnectDelay)
		return
	}

	if s.sendInit {
		s.serialPort.Write([]byte("km.buttons(1)\r"))
	}

	s.serialPort.SetReadTimeout(time.Millisecond)
	s.reconnectAttempts = 0
	s.log("Reconnect successful")
}
