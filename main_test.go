package lib_dhctl

import "testing"

func TestFake(t *testing.T) {
	t.Run("Correct", func(t *testing.T) {
		t.Log("Correct")
	})

	t.Run("Incorrect", func(t *testing.T) {
		t.Errorf("not pass")
	})
}
