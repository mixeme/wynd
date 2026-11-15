//go:build dev || desktop || bindings

package main

import (
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/workshop"
)

func (a *App) Profiles() ([]workshop.ProfileState, error) {
	if a.profileError != nil {
		return nil, a.profileError
	}
	return a.workshop.Profiles(), nil
}

func (a *App) SaveProfile(p workshop.ServerProfile) (workshop.ServerProfile, error) {
	if a.profileError != nil {
		return workshop.ServerProfile{}, a.profileError
	}
	return a.workshop.SaveProfile(p)
}

// SetInstallRoot задаёт каталог установки перед планом (план «другой каталог»).
func (a *App) SetInstallRoot(id, root string) error {
	if a.profileError != nil {
		return a.profileError
	}
	return a.workshop.SetInstallRoot(id, root)
}

func (a *App) SetAutoConnect(id string, on bool) error {
	if a.profileError != nil {
		return a.profileError
	}
	return a.workshop.SetAutoConnect(id, on)
}

func (a *App) RemoveProfile(id string) error {
	if a.profileError != nil {
		return a.profileError
	}
	return a.workshop.RemoveProfile(id)
}

func (a *App) profile(id string) workshop.ServerProfile {
	for _, p := range a.workshop.Profiles() {
		if p.ID == id {
			return p.ServerProfile
		}
	}
	return workshop.ServerProfile{}
}

func (a *App) moveReady(id string) error {
	if a.profileError != nil {
		return a.profileError
	}
	return a.workshop.BindMoveEndpoint(id)
}

func profileCall[T any](a *App, id string, allowRunning bool, fn func(*workshop.Wizard) T) (T, error) {
	return profileCallWith(a, id, allowRunning, false, fn)
}

func profileMoveCall[T any](a *App, id string, allowRunning bool, fn func(*workshop.Wizard) T) (T, error) {
	return profileCallWith(a, id, allowRunning, true, fn)
}

func profileCallWith[T any](a *App, id string, allowRunning, move bool, fn func(*workshop.Wizard) T) (T, error) {
	var result T
	if a.profileError != nil {
		return result, a.profileError
	}
	with := a.workshop.WithProfile
	if move {
		with = a.workshop.WithMoveProfile
	}
	var operationErr error
	err := with(id, allowRunning, func(w *workshop.Wizard, _ workshop.ServerProfile) {
		result = fn(w)
		if connect, ok := any(result).(workshop.ConnectResult); move && ok && connect.Status == workshop.StatusConnected {
			operationErr = a.workshop.BindMoveEndpoint(id)
		}
	})
	if err != nil {
		return result, err
	}
	if operationErr != nil {
		return result, operationErr
	}
	var check *workshop.ServerCheck
	switch r := any(result).(type) {
	case workshop.ServerPage:
		if r.OK {
			check = pageCheck(r)
		}
	case workshop.ServiceResult:
		if r.Page.OK {
			check = pageCheck(r.Page)
		}
	case workshop.InspectResult:
		if r.OK {
			check = &workshop.ServerCheck{At: time.Now().UTC()}
		}
	}
	if check != nil {
		err = a.workshop.RememberCheck(id, *check)
	}
	return result, err
}

func pageCheck(p workshop.ServerPage) *workshop.ServerCheck {
	return &workshop.ServerCheck{At: time.Now().UTC(), Domain: p.State.Domain, Version: p.State.Version, Installed: p.State.Installed, Running: p.State.Running, FreeBytes: p.State.FreeBytes, Root: p.State.Root}
}
