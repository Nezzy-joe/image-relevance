package worker

import (
	"math"
	"testing"
	"time"
)

func TestRetryDelayExponentialAndCapped(t *testing.T) {
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 10: 5 * time.Minute} {
		if got := retryDelay(attempt); got != want {
			t.Errorf("retryDelay(%d)=%s, want %s", attempt, got, want)
		}
	}
}

func TestFormatVector(t *testing.T) {
	got, err := formatVector([]float32{0.25, -0.5, 1})
	if err != nil || got != "[0.25,-0.5,1]" {
		t.Fatalf("formatVector()=(%q,%v)", got, err)
	}
	if _, err := formatVector([]float32{float32(math.NaN())}); err == nil {
		t.Fatal("formatVector accepted NaN")
	}
}
