package option

import (
	"testing"
)

func TestOption_PutGet(t *testing.T) {
	o := NewOption()
	o.Put("key1", "value1")
	if o.Get("key1") != "value1" {
		t.Errorf("Expected value1, got %s", o.Get("key1"))
	}
	if o.Get("nonexistent") != "" {
		t.Errorf("Expected empty string, got %s", o.Get("nonexistent"))
	}
}

func TestOption_GetAsInt(t *testing.T) {
	o := NewOption()
	o.Put("intKey", "123")
	val, err := o.GetAsInt("intKey")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != 123 {
		t.Errorf("Expected 123, got %d", val)
	}

	o.Put("invalidInt", "abc")
	_, err = o.GetAsInt("invalidInt")
	if err == nil {
		t.Error("Expected error for invalid int")
	}
}

func TestOption_GetAsBool(t *testing.T) {
	o := NewOption()
	o.Put("boolKey", "true")
	val, err := o.GetAsBool("boolKey")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != true {
		t.Errorf("Expected true, got %v", val)
	}

	o.Put("boolKey2", "false")
	val, err = o.GetAsBool("boolKey2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != false {
		t.Errorf("Expected false, got %v", val)
	}
}

func TestParseUnitNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"10", 10, false},
		{"1K", 1024, false},
		{"1M", 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{" 500k ", 500 * 1024, false},
		{"", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseUnitNumber(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseUnitNumber(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseUnitNumber(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
