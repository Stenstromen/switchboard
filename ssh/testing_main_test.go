package ssh

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if IsAskpass() {
		os.Exit(RunAskpass())
	}
	os.Exit(m.Run())
}
