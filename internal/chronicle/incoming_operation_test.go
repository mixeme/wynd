package chronicle_test

import (
	"gitea.mixdep.ru/mix/wynd/internal/chronicle"
	"testing"
)

func TestIncomingPlaceOperationSurvivesReissue(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current-source", true: "legacy-source"}[legacy], func(t *testing.T) {
			e := newTestEnv(t)
			e.seedAccount("owner")
			frozen := e.at(0)
			card := chronicle.IncomingCard{SourceURL: "https://source.example", SourceCircleID: "circle-one", CircleName: "Семья", Bytes: 1000, Posts: 2, Members: 3, FrozenAt: &frozen, ExpiresAt: e.at(3)}
			if legacy {
				card.SourceCircleID = ""
			}
			first, err := e.ch.PutIncomingDraft(e.ctx, "owner", "https://source.example/move/first", card, e.at(1))
			if err != nil {
				t.Fatal(err)
			}
			second, err := e.ch.PutIncomingDraft(e.ctx, "owner", "https://source.example/move/second", card, e.at(2))
			if err != nil {
				t.Fatal(err)
			}
			if first.ID == second.ID || first.OperationKey == "" || first.OperationKey != second.OperationKey {
				t.Fatalf("reissue lost operation: %+v %+v", first, second)
			}
			// Новый сервер исходного круга тоже не теряет прежнюю связь формы.
			card.SourceCircleID = "circle-one"
			third, err := e.ch.PutIncomingDraft(e.ctx, "owner", "https://source.example/move/third", card, e.at(2))
			if err != nil || third.OperationKey != first.OperationKey {
				t.Fatalf("source upgrade lost operation: %+v %v", third, err)
			}
			card.SourceCircleID = "another-circle"
			other, err := e.ch.PutIncomingDraft(e.ctx, "owner", "https://source.example/move/other", card, e.at(2))
			if err != nil || other.OperationKey == third.OperationKey {
				t.Fatalf("same name shared operation: %+v %v", other, err)
			}
		})
	}
}
