package relver

import "testing"

func TestNumber(t *testing.T) {
	for _, tag := range []string{"0.4.1", "10.2.30", "0.0.0"} {
		if _, ok := Number(tag); !ok {
			t.Fatalf("%q не разобран", tag)
		}
	}
	for _, tag := range []string{"", "0.4", "0.4.1.2", "v0.4.1", "0.4.x", "0.4.1234567"} {
		if _, ok := Number(tag); ok {
			t.Fatalf("%q разобран, а не должен", tag)
		}
	}
	if n, _ := Number("0.4.1"); n != [3]int{0, 4, 1} {
		t.Fatalf("0.4.1 = %v", n)
	}
}

func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.4.1", "0.4.2", true},
		{"0.4.2", "0.4.1", false},
		{"0.4.1", "0.4.1", false},
		{"0.9.9", "1.0.0", true},
		{"1.0.0", "0.9.9", false},
	}
	for _, c := range cases {
		a, _ := Number(c.a)
		b, _ := Number(c.b)
		if got := Less(a, b); got != c.want {
			t.Fatalf("Less(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
