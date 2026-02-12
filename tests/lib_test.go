package lib_test

import (
	"testing"

	Macku "github.com/Auchrio/Makcu-go-lib"
)

func TestHelloWorld(t *testing.T) {
	result := Macku.HelloWorld()
	expected := "Hello, World!"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
