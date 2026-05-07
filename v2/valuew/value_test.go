package valuew

import (
	"testing"
	"time"
)

func TestCoalesceString(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"first non-empty", []string{"hello", "world"}, "hello"},
		{"first empty uses second", []string{"", "fallback"}, "fallback"},
		{"all empty returns zero", []string{"", ""}, ""},
		{"single non-empty", []string{"only"}, "only"},
		{"single empty", []string{""}, ""},
		{"no args returns zero", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Coalesce(tt.vals...); got != tt.want {
				t.Errorf("Coalesce(%v) = %q, want %q", tt.vals, got, tt.want)
			}
		})
	}
}

func TestCoalesceInt(t *testing.T) {
	tests := []struct {
		name string
		vals []int
		want int
	}{
		{"first non-zero", []int{5, 10}, 5},
		{"first zero uses second", []int{0, 42}, 42},
		{"all zero returns zero", []int{0, 0}, 0},
		{"no args returns zero", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Coalesce(tt.vals...); got != tt.want {
				t.Errorf("Coalesce(%v) = %d, want %d", tt.vals, got, tt.want)
			}
		})
	}
}

func TestCoalesceDuration(t *testing.T) {
	d1 := 5 * time.Second
	d2 := 10 * time.Second

	if got := Coalesce(d1, d2); got != d1 {
		t.Errorf("Coalesce with first non-zero = %v, want %v", got, d1)
	}
	if got := Coalesce(time.Duration(0), d2); got != d2 {
		t.Errorf("Coalesce with first zero = %v, want %v", got, d2)
	}
}

func TestCoalesceBool(t *testing.T) {
	if got := Coalesce(true, false); got != true {
		t.Errorf("Coalesce(true, false) = %v, want true", got)
	}
	if got := Coalesce(false, true); got != true {
		t.Errorf("Coalesce(false, true) = %v, want true", got)
	}
	if got := Coalesce(false, false); got != false {
		t.Errorf("Coalesce(false, false) = %v, want false", got)
	}
}

func TestCoalesceFloat(t *testing.T) {
	if got := Coalesce(0.7, 0.3); got != 0.7 {
		t.Errorf("Coalesce(0.7, 0.3) = %v, want 0.7", got)
	}
	if got := Coalesce(0.0, 0.3); got != 0.3 {
		t.Errorf("Coalesce(0.0, 0.3) = %v, want 0.3", got)
	}
}