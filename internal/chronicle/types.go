package chronicle

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a referenced row does not exist.
var ErrNotFound = errors.New("chronicle: not found")

// ErrForbidden is returned when the actor lacks permission.
var ErrForbidden = errors.New("chronicle: forbidden")

// ErrInvalid is returned for invalid input or state transitions.
var ErrInvalid = errors.New("chronicle: invalid")

// EditWindow describes circle/content edit policy.
// Chronicle (0) forbids edits; nil duration means unlimited.
type EditWindow struct {
	Seconds *int64
}

func ChronicleWindow() EditWindow {
	zero := int64(0)
	return EditWindow{Seconds: &zero}
}

func UnlimitedWindow() EditWindow {
	return EditWindow{Seconds: nil}
}

func DurationWindow(d time.Duration) EditWindow {
	sec := int64(d / time.Second)
	return EditWindow{Seconds: &sec}
}

func (w EditWindow) IsChronicle() bool {
	return w.Seconds != nil && *w.Seconds == 0
}

func (w EditWindow) IsUnlimited() bool {
	return w.Seconds == nil
}

func (w EditWindow) EditableUntil(publishedAt time.Time) *time.Time {
	if w.IsChronicle() {
		return nil
	}
	if w.IsUnlimited() {
		t := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
		return &t
	}
	t := publishedAt.Add(time.Duration(*w.Seconds) * time.Second)
	return &t
}

func (w EditWindow) CanEdit(publishedAt, now time.Time) bool {
	until := w.EditableUntil(publishedAt)
	if until == nil {
		return false
	}
	return !now.After(*until)
}

type MembershipStatus string

const (
	StatusActive         MembershipStatus = "active"
	StatusLeftWithAccess MembershipStatus = "left_with_access"
	StatusGone           MembershipStatus = "gone"
)

type Circle struct {
	ID             string
	Name           string
	OwnerAccountID string
	Color          string
	EditWindow     EditWindow
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Identity struct {
	ID        string
	CircleID  string
	AccountID string
	Name      string
	CreatedAt time.Time
}

type Membership struct {
	ID          string
	CircleID    string
	AccountID   string
	IdentityID  string
	CanSettings bool
	Status      MembershipStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Span struct {
	ID           string
	MembershipID string
	StartedAt    time.Time
	EndedAt      *time.Time
	CanRead      bool
	CanWrite     bool
}

type Event struct {
	Seq             int64
	ID              string
	CircleID        string
	Type            string
	IsService       bool
	ActorIdentityID string
	ActorName       string
	TargetID        string
	Payload         string
	Summary         string
	CreatedAt       time.Time
}

type Post struct {
	ID            string
	CircleID      string
	EventSeq      int64
	IdentityID    string
	AuthorName    string
	Body          string
	EntryDate     string
	CapturedAt    *time.Time
	CreatedAt     time.Time
	EditWindow    EditWindow
	EditableUntil *time.Time
	Deleted       bool
}

type Comment struct {
	ID            string
	CircleID      string
	PostID        string
	EventSeq      int64
	IdentityID    string
	AuthorName    string
	Body          string
	CreatedAt     time.Time
	EditWindow    EditWindow
	EditableUntil *time.Time
	Deleted       bool
}

type Reaction struct {
	ID            string
	CircleID      string
	PostID        string
	EventSeq      int64
	IdentityID    string
	AuthorName    string
	Emoji         string
	CreatedAt     time.Time
	EditWindow    EditWindow
	EditableUntil *time.Time
	Deleted       bool
}

type Day struct {
	CircleID    string
	EntryDate   string
	Title       string
	CoverPostID string
	CoverBlobID string
}

type CreateCircleInput struct {
	Name           string
	OwnerAccountID string
	OwnerName      string
	Color          string
	EditWindow     EditWindow
	Now            time.Time
}

type JoinInput struct {
	CircleID  string
	AccountID string
	Name      string
	Now       time.Time
}

type PostInput struct {
	CircleID   string
	AccountID  string
	Body       string
	EntryDate  string
	CapturedAt *time.Time
	Now        time.Time
}

type CommentInput struct {
	CircleID  string
	AccountID string
	PostID    string
	Body      string
	Now       time.Time
}

type ReactionInput struct {
	CircleID  string
	AccountID string
	PostID    string
	Emoji     string
	Now       time.Time
}

type DayTitleInput struct {
	CircleID  string
	AccountID string
	EntryDate string
	Title     string
	Now       time.Time
}

type DayCoverInput struct {
	CircleID  string
	AccountID string
	EntryDate string
	PostID    string
	BlobID    string
	Now       time.Time
}
