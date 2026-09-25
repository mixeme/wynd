package chronicle

import "testing"

func TestFindMentionedNames(t *testing.T) {
	names := map[string]string{
		"Аня":   "a1",
		"Антон": "a2",
		"Кот":   "a3",
	}
	got := findMentionedNames("@Аня, привет @Антон и @Кот", names)
	if len(got) != 3 {
		t.Fatalf("expected 3 mentions, got %v", got)
	}
	got = findMentionedNames("письмо @Антону", names)
	if len(got) != 0 {
		t.Fatalf("partial name should not match: %v", got)
	}
	got = findMentionedNames("@Антон и @Аня", names)
	if len(got) != 2 || got[0] != "Антон" {
		t.Fatalf("expected Антон first (longest), got %v", got)
	}
}

func TestCheckByteLen(t *testing.T) {
	if err := checkByteLen("привет", MaxTextBytes); err != nil {
		t.Fatal(err)
	}
	long := string(make([]byte, MaxTextBytes+1))
	if err := checkByteLen(long, MaxTextBytes); err != ErrTooLong {
		t.Fatalf("expected ErrTooLong, got %v", err)
	}
}
