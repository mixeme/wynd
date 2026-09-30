package auth

import (
	"testing"
	"time"
)

func TestInviteArrivesWhenEventsOff(t *testing.T) {
	prefs := DefaultNotifyPrefs()
	now := time.Now().UTC()
	if prefs.Events {
		t.Fatal("events default on: invite regression would not be visible")
	}
	if !NotifyPrefAllows(prefs, "invite", now) {
		t.Fatal("invite must arrive when events are off")
	}
	if NotifyPrefAllows(prefs, "event", now) {
		t.Fatal("events stay off by default")
	}
	until := now.Add(time.Hour).Format(time.RFC3339)
	prefs.MuteUntil = &until
	if NotifyPrefAllows(prefs, "invite", now) {
		t.Fatal("muted invite must wait")
	}
}
