package class

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func dt(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func TestParseValid(t *testing.T) {
	tests := []struct {
		in   string
		want Mail
	}{
		{"it2601@sspu-opava.cz", Mail{IT, 26, 1}},
		{"UO2512@SSPU-OPAVA.CZ", Mail{UO, 25, 12}},
		{"sva2401@sspu-opava.cz", Mail{SvA, 24, 1}},
		{"svb2302@sspu-opava.cz", Mail{SvB, 23, 2}},
		{"  it2601@sspu-opava.cz ", Mail{IT, 26, 1}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		in   string
		want error
	}{
		{"foo2601@sspu-opava.cz", ErrFormat},
		{"it26@sspu-opava.cz", ErrFormat},
		{"it260011@sspu-opava.cz", ErrFormat},
		{"it26x1@sspu-opava.cz", ErrFormat},
		{"it2601@gmail.com", ErrDomain},
		{"it2601", ErrFormat},
		{"@sspu-opava.cz", ErrFormat},
		{"", ErrFormat},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		if !errors.Is(err, tt.want) {
			t.Errorf("Parse(%q) error = %v, want %v", tt.in, err, tt.want)
		}
	}
}

func TestRole(t *testing.T) {
	tests := []struct {
		name   string
		mail   string
		now    time.Time
		want   string
		wantOK bool
	}{
		{"grade 1 mid year", "it2501@sspu-opava.cz", dt(2025, time.October, 15), "IT1", true},
		{"june is still old school year", "it2501@sspu-opava.cz", dt(2026, time.June, 1), "IT1", true},
		{"rollover september", "it2501@sspu-opava.cz", dt(2026, time.September, 1), "IT2", true},
		{"incoming mail before school year start rejected", "it2601@sspu-opava.cz", dt(2026, time.June, 1), "", false},
		{"incoming mail valid from september", "it2601@sspu-opava.cz", dt(2026, time.September, 1), "IT1", true},
		{"uo two years after enrollment", "uo2501@sspu-opava.cz", dt(2027, time.September, 1), "Uo3", true},
		{"uo one year after enrollment", "uo2501@sspu-opava.cz", dt(2026, time.September, 1), "Uo2", true},
		{"sva final grade in june", "sva2201@sspu-opava.cz", dt(2026, time.June, 1), "Sv4A", true},
		{"sva graduates in september", "sva2201@sspu-opava.cz", dt(2026, time.September, 1), RoleAbsolvent, true},
		{"old mail is absolvent", "svb1901@sspu-opava.cz", dt(2026, time.June, 1), RoleAbsolvent, true},
		{"way old mail is absolvent", "it1001@sspu-opava.cz", dt(2026, time.June, 1), RoleAbsolvent, true},
		{"svb grade 3", "svb2302@sspu-opava.cz", dt(2026, time.January, 15), "Sv3B", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mail, err := Parse(tt.mail)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.mail, err)
			}
			got, ok := mail.Role(tt.now)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Role at %s = (%q, %v), want (%q, %v)", tt.now, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAllRoleNames(t *testing.T) {
	names := AllRoleNames()
	if len(names) != 17 {
		t.Fatalf("len(AllRoleNames()) = %d, want 17", len(names))
	}
	if !slices.Contains(names, "IT1") || !slices.Contains(names, "Uo4") ||
		!slices.Contains(names, "Sv2A") || !slices.Contains(names, "Sv2B") ||
		!slices.Contains(names, RoleAbsolvent) {
		t.Errorf("missing expected role names in %v", names)
	}
	if slices.Contains(names, "") {
		t.Errorf("empty role name in %v", names)
	}
}
