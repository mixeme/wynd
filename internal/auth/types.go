package auth

import (
	"context"
	"database/sql"
	"time"
)

const (
	// AdminSentinelEmail is the local account bound to admin sessions.
	// It cannot register, request a login code, or accept invites.
	AdminSentinelEmail = "admin@wynd.local"
)

type RegistrationMode string

const (
	ModeOpen    RegistrationMode = "open"
	ModeInvite  RegistrationMode = "invite"
	ModeClosed  RegistrationMode = "closed"
	ModeDefault                  = ModeInvite
)

type InviteKind string

const (
	InviteSingle InviteKind = "single"
	InviteMulti  InviteKind = "multi"
)

type SessionKind string

const (
	SessionParticipant SessionKind = "participant"
	SessionAdmin       SessionKind = "admin"
)

type Flow string

const (
	FlowLogin    Flow = "login"
	FlowRegister Flow = "register"
	FlowInvite   Flow = "invite"
)

type InstanceInfo struct {
	Name             string           `json:"name"`
	Version          string           `json:"version"`
	RegistrationMode RegistrationMode `json:"registration_mode"`
	Loopback         bool             `json:"loopback"`
	Bootstrapped     bool             `json:"bootstrapped"`
}

type Account struct {
	ID        string
	Email     string
	CreatedAt time.Time
	Blocked   bool
}

type Invite struct {
	ID                 string
	Token              string
	CircleID           string
	Kind               InviteKind
	MaxUses            int
	Uses               int
	ExpiresAt          time.Time
	RevokedAt          *time.Time
	CreatedByAccountID string
	TargetAccountID    string
	CreatedAt          time.Time
}

// IsServer reports an invite to the server without a circle.
func (i Invite) IsServer() bool {
	return i.CircleID == ""
}

type Session struct {
	Token     string
	AccountID string
	Kind      SessionKind
	ExpiresAt time.Time
	CreatedAt time.Time
}

type BootstrapInput struct {
	Token        string
	InstanceName string
	Password     string
	ClientIP     string
	Now          time.Time
	// InTx, если задан, выполняется внутри транзакции установки: первичная
	// настройка инстанса (например SMTP-релей) применяется вместе с флагом
	// bootstrapped или не применяется вовсе.
	InTx func(context.Context, *sql.Tx) error
}

type CreateInviteInput struct {
	CircleID           string
	Kind               InviteKind
	MaxUses            int
	TTL                time.Duration
	CreatedByAccountID string
	TargetAccountID    string
	Now                time.Time
}

type CreateMemberInviteInput struct {
	CircleID           string
	TargetAccountID    string
	CreatedByAccountID string
	Now                time.Time
}

type AcceptInviteInput struct {
	Token    string
	Email    string
	Name     string
	ClientIP string
	Now      time.Time
}

type RegisterInput struct {
	Email    string
	ClientIP string
	Now      time.Time
}

type RequestCodeInput struct {
	Email    string
	ClientIP string
	Now      time.Time
}

type VerifyInput struct {
	Email    string
	Code     string
	ClientIP string
	Now      time.Time
}

type VerifyResult struct {
	Session         Session
	Account         Account
	PendingCircleID string
}

type AdminLoginInput struct {
	Password string
	ClientIP string
	Now      time.Time
}

type CreateServerInviteInput struct {
	Kind               InviteKind
	MaxUses            int
	TTL                time.Duration
	// Только у ссылки админа на сервер: «без ограничений» (для multi) и
	// «без срока». Ссылка участника в круг их не получает.
	UnlimitedUses bool
	NoExpiry      bool
	CreatedByAccountID string
	Now                time.Time
}
