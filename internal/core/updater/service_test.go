package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"v0.2.0", "v0.1.0", 1},
		{"v0.1.0", "v0.1.0-alpha", 1},
		{"v0.1.0-alpha", "v0.1.0-alpha-2-gabcdef", -1},
		{"v1.0.0", "dev", 0},
	}
	for _, test := range tests {
		got := compareVersions(test.left, test.right)
		if got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}
