package inspector

import "testing"

func TestSameIPSet(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"同序", []string{"1.1.1.1", "2.2.2.2"}, []string{"1.1.1.1", "2.2.2.2"}, true},
		{"乱序同集合", []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, []string{"3.3.3.3", "1.1.1.1", "2.2.2.2"}, true},
		{"数量不同", []string{"1.1.1.1", "2.2.2.2"}, []string{"1.1.1.1"}, false},
		{"内容不同", []string{"1.1.1.1", "2.2.2.2"}, []string{"1.1.1.1", "9.9.9.9"}, false},
	}
	for _, c := range cases {
		if got := sameIPSet(c.a, c.b); got != c.want {
			t.Errorf("%s: sameIPSet = %v, want %v", c.name, got, c.want)
		}
	}
}
