//go:build windows

package Macku

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

type point struct {
	X int32
	Y int32
}

func getCursorPos() (int, int, error) {
	var pt point
	r, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos failed: %v", err)
	}
	return int(pt.X), int(pt.Y), nil
}

func getMouseSpeedMultiplier() (float64, error) {
	const spiGetMouseSpeed = 0x0070
	var speed uint32
	r, _, err := procSystemParametersInfo.Call(
		uintptr(spiGetMouseSpeed),
		0,
		uintptr(unsafe.Pointer(&speed)),
		0,
	)
	if r == 0 {
		return 0, fmt.Errorf("SystemParametersInfoW failed: %v", err)
	}
	return float64(speed) / 10.0, nil
}

// MoveAbs moves the mouse cursor to an absolute screen position by issuing
// incremental relative moves, compensating for the Windows pointer-speed setting.
// Speed is clamped to 1–14. This function is only available on Windows.
func (m *Mouse) MoveAbs(target [2]int, speed int, waitMs int) error {
	multiplier, err := getMouseSpeedMultiplier()
	if err != nil {
		return err
	}

	endX, endY := target[0], target[1]
	speed = clamp(speed, 1, 14)

	for {
		cx, cy, err := getCursorPos()
		if err != nil {
			return err
		}

		dx, dy := endX-cx, endY-cy
		if absInt(dx) <= 1 && absInt(dy) <= 1 {
			break
		}

		moveX := clamp(int(float64(dx)/multiplier), -speed, speed)
		moveY := clamp(int(float64(dy)/multiplier), -speed, speed)

		_, err = m.transport.SendCommand(fmt.Sprintf("km.move(%d,%d)", moveX, moveY), false, 0)
		if err != nil {
			return err
		}
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	return nil
}
