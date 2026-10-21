package input

import "testing"

func TestParseMinMaxInt(t *testing.T) {
	args := []Arg{{Name: "n", Type: TypeInt, Min: 1, Max: 10}}
	if _, err := Parse("n", args, []string{"--n", "0"}); err == nil {
		t.Fatal("expected min error")
	}
	if _, err := Parse("n", args, []string{"--n", "11"}); err == nil {
		t.Fatal("expected max error")
	}
	in, err := Parse("n", args, []string{"--n", "5"})
	if err != nil {
		t.Fatal(err)
	}
	if in.Int("n") != 5 {
		t.Fatalf("got %d", in.Int("n"))
	}
}

func TestParseMinMaxString(t *testing.T) {
	args := []Arg{{Name: "s", Type: TypeString, Min: 2, Max: 4}}
	if _, err := Parse("s", args, []string{"--s", "a"}); err == nil {
		t.Fatal("expected min error")
	}
	if _, err := Parse("s", args, []string{"--s", "abcde"}); err == nil {
		t.Fatal("expected max error")
	}
}
