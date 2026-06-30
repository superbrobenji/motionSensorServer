package nodeauth

import (
	"testing"
)

func TestMacToString_ColonSeparated(t *testing.T) {
	mac := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	got := macToString(mac)
	want := "aa:bb:cc:dd:ee:ff"
	if got != want {
		t.Errorf("macToString = %q, want %q", got, want)
	}
}

func TestParseMAC_AcceptsBothFormats(t *testing.T) {
	cases := []struct {
		input string
		want  [6]byte
	}{
		{"aa:bb:cc:dd:ee:ff", [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}},
		{"aabbccddeeff", [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}},
	}
	for _, tc := range cases {
		got, err := ParseMAC(tc.input)
		if err != nil {
			t.Errorf("ParseMAC(%q): %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseMAC(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseMAC_RejectsInvalid(t *testing.T) {
	_, err := ParseMAC("not-a-mac")
	if err == nil {
		t.Error("ParseMAC(invalid) = nil error, want error")
	}
}
