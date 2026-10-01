package chronicle

import "testing"

// Инвариант (план 46, A5): строка журнала — по роду участника; род не
// выбран — без глагола.
func TestSummariesFollowGender(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{summaryMemberJoined(GenderFemale, "Аня"), "Аня вступила в круг"},
		{summaryMemberJoined(GenderMale, "Боря"), "Боря вступил в круг"},
		{summaryMemberJoined(GenderNone, "Аня"), "В круге: Аня"},
		{summaryMemberLeft(GenderFemale, "Аня"), "Аня покинула круг"},
		{summaryPostCreated(GenderFemale, "Аня"), "Аня опубликовала запись"},
		{summaryPostCreated(GenderNone, "Аня"), "Новая запись: Аня"},
		{summaryCommentCreated(GenderFemale, "Аня"), "Аня оставила комментарий"},
		{summaryReactionSet(GenderFemale, "Аня", "❤"), "Аня поставила реакцию ❤"},
		{summaryDayTitled(GenderFemale, "Мама", "Плёнки", "2026-08-06"), "Мама назвала 6 августа «Плёнки»"},
		{summaryDayTitled(GenderNone, "Мама", "Плёнки", "2026-08-06"), "Название для 6 августа — «Плёнки»: Мама"},
		{summaryDayCoverSet(GenderMale, "Кот", "2026-08-06"), "Кот выбрал обложку для 6 августа"},
		{summarySettingsGranted(GenderFemale, "Аня", "Боб"), "Аня дала право менять настройки: Боб"},
		{summarySettingsRevoked(GenderNone, "Аня", "Боб"), "Аня: без права менять настройки — Боб"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if _, err := ParseGender("x"); err != ErrInvalid {
		t.Fatalf("ParseGender(x) = %v, want ErrInvalid", err)
	}
}
