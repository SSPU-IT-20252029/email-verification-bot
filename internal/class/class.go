package class

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sspu-verifier/internal/schoolyear"
)

type Type string

const (
	IT  Type = "it"
	UO  Type = "uo"
	SvA Type = "sva"
	SvB Type = "svb"
)

const Domain = "sspu-opava.cz"

const RoleAbsolvent = "Absolvent"

var (
	ErrDomain = errors.New("domain")
	ErrFormat = errors.New("format")
)

var localRe = regexp.MustCompile(`^(it|uo|sva|svb)([0-9]{2})([0-9]{2})$`)

type Mail struct {
	Type  Type
	Year  int
	Index int
}

func Parse(addr string) (Mail, error) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	local, domain, ok := strings.Cut(addr, "@")
	if !ok || local == "" {
		return Mail{}, fmt.Errorf("%w: chybí @ nebo místní část", ErrFormat)
	}
	if domain != Domain {
		return Mail{}, fmt.Errorf("%w: očekávám doménu %s", ErrDomain, Domain)
	}
	m := localRe.FindStringSubmatch(local)
	if m == nil {
		return Mail{}, fmt.Errorf("%w: očekávám tvar it2601, např. it2601@%s", ErrFormat, Domain)
	}
	year, _ := strconv.Atoi(m[2])
	index, _ := strconv.Atoi(m[3])
	return Mail{Type: Type(m[1]), Year: year, Index: index}, nil
}

func (m Mail) FullYear() int {
	return 2000 + m.Year
}

func (m Mail) String() string {
	return fmt.Sprintf("%s%02d%02d@%s", m.Type, m.Year, m.Index, Domain)
}

func (m Mail) Role(now time.Time) (string, bool) {
	grade := schoolyear.Start(now) - m.FullYear() + 1
	switch {
	case grade >= 1 && grade <= 4:
		return roleName(m.Type, grade), true
	case grade >= 5:
		return RoleAbsolvent, true
	default:
		return "", false
	}
}

func roleName(t Type, grade int) string {
	switch t {
	case IT:
		return fmt.Sprintf("IT%d", grade)
	case UO:
		return fmt.Sprintf("Uo%d", grade)
	case SvA:
		return fmt.Sprintf("Sv%dA", grade)
	case SvB:
		return fmt.Sprintf("Sv%dB", grade)
	default:
		return ""
	}
}

func AllRoleNames() []string {
	names := make([]string, 0, 17)
	for g := 1; g <= 4; g++ {
		names = append(names,
			roleName(IT, g),
			roleName(UO, g),
			roleName(SvA, g),
			roleName(SvB, g),
		)
	}
	return append(names, RoleAbsolvent)
}
