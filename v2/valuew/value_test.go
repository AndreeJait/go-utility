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

// --- Pointer Helpers ---

func TestPtr(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"int zero", 0, 0},
		{"int positive", 42, 42},
		{"int negative", -1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ptr(tt.input)
			if got == nil {
				t.Fatal("Ptr returned nil")
			}
			if *got != tt.want {
				t.Errorf("Ptr(%d) = %d, want %d", tt.input, *got, tt.want)
			}
		})
	}

	t.Run("string", func(t *testing.T) {
		p := Ptr("hello")
		if p == nil || *p != "hello" {
			t.Errorf("Ptr(\"hello\") = %v, want \"hello\"", p)
		}
	})

	t.Run("bool", func(t *testing.T) {
		p := Ptr(true)
		if p == nil || *p != true {
			t.Errorf("Ptr(true) = %v, want true", p)
		}
	})

	t.Run("float64", func(t *testing.T) {
		p := Ptr(3.14)
		if p == nil || *p != 3.14 {
			t.Errorf("Ptr(3.14) = %v, want 3.14", p)
		}
	})
}

func TestDeref(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		v := 42
		if got := Deref(&v); got != 42 {
			t.Errorf("Deref(&42) = %d, want 42", got)
		}
	})

	t.Run("nil pointer with fallback", func(t *testing.T) {
		var p *int
		if got := Deref(p, 99); got != 99 {
			t.Errorf("Deref(nil, 99) = %d, want 99", got)
		}
	})

	t.Run("nil pointer without fallback returns zero", func(t *testing.T) {
		var p *int
		if got := Deref(p); got != 0 {
			t.Errorf("Deref(nil) = %d, want 0", got)
		}
	})

	t.Run("string with fallback", func(t *testing.T) {
		var p *string
		if got := Deref(p, "default"); got != "default" {
			t.Errorf("Deref(nil, \"default\") = %q, want \"default\"", got)
		}
	})

	t.Run("non-nil ignores fallback", func(t *testing.T) {
		v := "actual"
		if got := Deref(&v, "fallback"); got != "actual" {
			t.Errorf("Deref(&\"actual\", \"fallback\") = %q, want \"actual\"", got)
		}
	})
}

func TestDerefZero(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		v := 42
		if got := DerefZero(&v); got != 42 {
			t.Errorf("DerefZero(&42) = %d, want 42", got)
		}
	})

	t.Run("nil pointer returns zero", func(t *testing.T) {
		var p *int
		if got := DerefZero(p); got != 0 {
			t.Errorf("DerefZero(nil) = %d, want 0", got)
		}
	})

	t.Run("nil string pointer returns empty", func(t *testing.T) {
		var p *string
		if got := DerefZero(p); got != "" {
			t.Errorf("DerefZero(nil) = %q, want \"\"", got)
		}
	})
}

func TestIsNil(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		var p *int
		if !IsNil(p) {
			t.Error("IsNil(nil) = false, want true")
		}
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		v := 42
		if IsNil(&v) {
			t.Error("IsNil(&42) = true, want false")
		}
	})
}

func TestEqualPtr(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		if !EqualPtr[int](nil, nil) {
			t.Error("EqualPtr(nil, nil) = false, want true")
		}
	})

	t.Run("one nil", func(t *testing.T) {
		v := 42
		if EqualPtr(&v, nil) {
			t.Error("EqualPtr(&42, nil) = true, want false")
		}
		if EqualPtr(nil, &v) {
			t.Error("EqualPtr(nil, &42) = true, want false")
		}
	})

	t.Run("equal values", func(t *testing.T) {
		a, b := 42, 42
		if !EqualPtr(&a, &b) {
			t.Error("EqualPtr(&42, &42) = false, want true")
		}
	})

	t.Run("unequal values", func(t *testing.T) {
		a, b := 42, 99
		if EqualPtr(&a, &b) {
			t.Error("EqualPtr(&42, &99) = true, want false")
		}
	})
}

// --- Conversion Helpers ---

func TestToAny(t *testing.T) {
	if got := ToAny(42); got != 42 {
		t.Errorf("ToAny(42) = %v, want 42", got)
	}
	if got := ToAny("hello"); got != "hello" {
		t.Errorf("ToAny(\"hello\") = %v, want \"hello\"", got)
	}
}

func TestFromAny(t *testing.T) {
	t.Run("successful assertion", func(t *testing.T) {
		var v any = 42
		if got := FromAny[int](v); got != 42 {
			t.Errorf("FromAny[int](42) = %d, want 42", got)
		}
	})

	t.Run("failed assertion with fallback", func(t *testing.T) {
		var v any = "hello"
		if got := FromAny[int](v, 99); got != 99 {
			t.Errorf("FromAny[int](\"hello\", 99) = %d, want 99", got)
		}
	})

	t.Run("failed assertion without fallback returns zero", func(t *testing.T) {
		var v any = "hello"
		if got := FromAny[int](v); got != 0 {
			t.Errorf("FromAny[int](\"hello\") = %d, want 0", got)
		}
	})

	t.Run("nil value with fallback", func(t *testing.T) {
		if got := FromAny[string](nil, "default"); got != "default" {
			t.Errorf("FromAny[string](nil, \"default\") = %q, want \"default\"", got)
		}
	})
}

// --- Slice/Map Helpers ---

func TestCoalesceSlice(t *testing.T) {
	t.Run("first non-empty", func(t *testing.T) {
		a := []int{1, 2}
		b := []int{3, 4}
		if got := CoalesceSlice(a, b); len(got) != 2 || got[0] != 1 {
			t.Errorf("CoalesceSlice(%v, %v) = %v, want [1 2]", a, b, got)
		}
	})

	t.Run("first nil second non-empty", func(t *testing.T) {
		var a []int
		b := []int{3, 4}
		if got := CoalesceSlice(a, b); len(got) != 2 || got[0] != 3 {
			t.Errorf("CoalesceSlice(nil, %v) = %v, want [3 4]", b, got)
		}
	})

	t.Run("first empty second non-empty", func(t *testing.T) {
		a := []int{}
		b := []int{3, 4}
		if got := CoalesceSlice(a, b); len(got) != 2 || got[0] != 3 {
			t.Errorf("CoalesceSlice([], %v) = %v, want [3 4]", b, got)
		}
	})

	t.Run("all nil returns nil", func(t *testing.T) {
		var a, b []int
		if got := CoalesceSlice(a, b); got != nil {
			t.Errorf("CoalesceSlice(nil, nil) = %v, want nil", got)
		}
	})

	t.Run("no args returns nil", func(t *testing.T) {
		if got := CoalesceSlice[int](); got != nil {
			t.Errorf("CoalesceSlice() = %v, want nil", got)
		}
	})
}

func TestContains(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		if !Contains([]string{"a", "b", "c"}, "b") {
			t.Error("Contains([a b c], b) = false, want true")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if Contains([]string{"a", "b", "c"}, "d") {
			t.Error("Contains([a b c], d) = true, want false")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		if Contains([]string{}, "a") {
			t.Error("Contains([], a) = true, want false")
		}
	})

	t.Run("int slice", func(t *testing.T) {
		if !Contains([]int{1, 2, 3}, 2) {
			t.Error("Contains([1 2 3], 2) = false, want true")
		}
	})
}

func TestMapKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := MapKeys(m)
	if len(keys) != 3 {
		t.Errorf("MapKeys returned %d keys, want 3", len(keys))
	}
	// Check all keys are present
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !keySet[want] {
			t.Errorf("missing key %q", want)
		}
	}
}

func TestMapValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	values := MapValues(m)
	if len(values) != 3 {
		t.Errorf("MapValues returned %d values, want 3", len(values))
	}
	// Check all values are present
	sum := 0
	for _, v := range values {
		sum += v
	}
	if sum != 6 {
		t.Errorf("MapValues sum = %d, want 6", sum)
	}
}

func TestMapKeysEmpty(t *testing.T) {
	m := map[string]int{}
	keys := MapKeys(m)
	if len(keys) != 0 {
		t.Errorf("MapKeys of empty map = %v, want []", keys)
	}
}

func TestMapValuesEmpty(t *testing.T) {
	m := map[string]int{}
	values := MapValues(m)
	if len(values) != 0 {
		t.Errorf("MapValues of empty map = %v, want []", values)
	}
}