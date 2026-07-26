package agentclient

import (
	"math"
	"strconv"
	"testing"
)

func TestCheckedInt32(t *testing.T) {
	for _, value := range []int{math.MinInt32, -1, 0, 1, math.MaxInt32} {
		got, err := checkedInt32(value, "value")
		if err != nil {
			t.Fatalf("checkedInt32(%d) returned an error: %v", value, err)
		}
		if int64(got) != int64(value) {
			t.Fatalf("checkedInt32(%d) = %d", value, got)
		}
	}
}

func TestCheckedInt32RejectsOverflow(t *testing.T) {
	if strconv.IntSize <= 32 {
		t.Skip("int cannot represent values outside the int32 range")
	}
	for _, value := range []int{int(int64(math.MinInt32) - 1), int(int64(math.MaxInt32) + 1)} {
		if _, err := checkedInt32(value, "value"); err == nil {
			t.Fatalf("checkedInt32(%d) accepted an out-of-range value", value)
		}
	}
}
