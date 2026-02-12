package Macku

// MouseButton represents a mouse button identifier.
type MouseButton int

const (
	MouseButtonLeft   MouseButton = 0
	MouseButtonRight  MouseButton = 1
	MouseButtonMiddle MouseButton = 2
	MouseButton4      MouseButton = 3
	MouseButton5      MouseButton = 4
)

// String returns the lowercase name of the mouse button.
func (b MouseButton) String() string {
	switch b {
	case MouseButtonLeft:
		return "left"
	case MouseButtonRight:
		return "right"
	case MouseButtonMiddle:
		return "middle"
	case MouseButton4:
		return "mouse4"
	case MouseButton5:
		return "mouse5"
	default:
		return "unknown"
	}
}

// LockTarget identifies a lockable target (button or axis).
type LockTarget int

const (
	LockLeft LockTarget = iota
	LockRight
	LockMiddle
	LockMouse4
	LockMouse5
	LockX
	LockY
)

// ClickProfile defines a timing profile for human-like clicks.
type ClickProfile string

const (
	ProfileNormal   ClickProfile = "normal"
	ProfileFast     ClickProfile = "fast"
	ProfileSlow     ClickProfile = "slow"
	ProfileVariable ClickProfile = "variable"
	ProfileGaming   ClickProfile = "gaming"
)
