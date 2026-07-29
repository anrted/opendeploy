package stats

import "testing"

func TestProtectedProcessNames(t *testing.T) {
	for _, name := range []string{"systemd", "sshd", "opendeploy-agent", "opendeploy-core", "kworker/0:1"} {
		if protectedNameReason(name) == "" {
			t.Errorf("%q was not protected", name)
		}
	}
	if got := protectedNameReason("php-fpm"); got != "" {
		t.Fatalf("ordinary process was protected: %s", got)
	}
}

func TestProtectedProcessRejectsInitAndInvalidPID(t *testing.T) {
	if protectedProcessReason(1) == "" {
		t.Fatal("PID 1 was not protected")
	}
	if protectedProcessReason(0) == "" {
		t.Fatal("PID 0 was not rejected")
	}
}
