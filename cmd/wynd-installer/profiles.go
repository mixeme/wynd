//go:build dev || desktop || bindings

package main

import (
	"time"

	"gitea.mixdep.ru/mix/wynd/internal/installer"
)

func (a *App) Profiles() ([]installer.ProfileState, error) {
	if a.profileError != nil {
		return nil, a.profileError
	}
	return a.workshop.Profiles(), nil
}

func (a *App) SaveProfile(p installer.ServerProfile) (installer.ServerProfile, error) {
	if a.profileError != nil {
		return installer.ServerProfile{}, a.profileError
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

func (a *App) profile(id string) installer.ServerProfile {
	for _, p := range a.workshop.Profiles() {
		if p.ID == id {
			return p.ServerProfile
		}
	}
	return installer.ServerProfile{}
}

func (a *App) moveReady(id string) error {
	if a.profileError != nil {
		return a.profileError
	}
	return a.workshop.BindMoveEndpoint(id)
}

func profileCall[T any](a *App, id string, allowRunning bool, fn func(*installer.Wizard) T) (T, error) {
	return profileCallWith(a, id, allowRunning, false, fn)
}

func profileMoveCall[T any](a *App, id string, allowRunning bool, fn func(*installer.Wizard) T) (T, error) {
	return profileCallWith(a, id, allowRunning, true, fn)
}

func profileCallWith[T any](a *App, id string, allowRunning, move bool, fn func(*installer.Wizard) T) (T, error) {
	var result T
	if a.profileError != nil {
		return result, a.profileError
	}
	with := a.workshop.WithProfile
	if move {
		with = a.workshop.WithMoveProfile
	}
	var operationErr error
	err := with(id, allowRunning, func(w *installer.Wizard, _ installer.ServerProfile) {
		result = fn(w)
		if connect, ok := any(result).(installer.ConnectResult); move && ok && connect.Status == installer.StatusConnected {
			operationErr = a.workshop.BindMoveEndpoint(id)
		}
	})
	if err != nil {
		return result, err
	}
	if operationErr != nil {
		return result, operationErr
	}
	var check *installer.ServerCheck
	switch r := any(result).(type) {
	case installer.ServerPage:
		if r.OK {
			check = pageCheck(r)
		}
	case installer.ServiceResult:
		if r.Page.OK {
			check = pageCheck(r.Page)
		}
	case installer.InspectResult:
		if r.OK {
			check = &installer.ServerCheck{At: time.Now().UTC()}
		}
	}
	if check != nil {
		err = a.workshop.RememberCheck(id, *check)
	}
	return result, err
}

func pageCheck(p installer.ServerPage) *installer.ServerCheck {
	return &installer.ServerCheck{At: time.Now().UTC(), Domain: p.State.Domain, Version: p.State.Version, Installed: p.State.Installed, Running: p.State.Running, FreeBytes: p.State.FreeBytes, Root: p.State.Root}
}
