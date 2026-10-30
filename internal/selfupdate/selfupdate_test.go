package selfupdate

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.0.1", "1.0.0", true}, {"1.0.0", "1.0.0", false}, {"v0.9", "1.0.0-dev", false},
		{"v1.0.0", "1.0.0-dev", false}, {"2026.10.01", "2026.09.30", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%s,%s)=%v", c.a, c.b, got)
		}
	}
}
