package core

import (
	"errors"
	"fmt"
)

// NodeID is the immutable slug stored in filenames, parent_id, and relationships.
type NodeID string

// Status is the closed set of node status values.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusSpecified  Status = "specified"
	StatusAssigned   Status = "assigned"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusBlocked    Status = "blocked"
)

const (
	KindSync  = "sync"
	KindAsync = "async"
	KindData  = "data"
)

const schemaVersion = 1

var (
	// Validation sentinels. HTTP maps these; it does not reimplement the rules.
	ErrCycle           = errors.New("hierarchy cycle")
	ErrNotFound        = errors.New("node not found")
	ErrHasChildren     = errors.New("node has children")
	ErrNoSlug          = errors.New("name has no slug")
	ErrBadStatus       = errors.New("invalid status")
	ErrBadKind         = errors.New("invalid relationship kind")
	ErrMissingEndpoint = errors.New("relationship endpoint does not exist")
	ErrMissingParent   = errors.New("parent does not exist")
	ErrEmptyName       = errors.New("name must be non-empty")
	ErrEmptyType       = errors.New("type must be non-empty")

	errCycle           = ErrCycle
	errNotFound        = ErrNotFound
	errHasChildren     = ErrHasChildren
	errNoSlug          = ErrNoSlug
	errBadStatus       = ErrBadStatus
	errBadKind         = ErrBadKind
	errMissingEndpoint = ErrMissingEndpoint
	errMissingParent   = ErrMissingParent
	errEmptyName       = ErrEmptyName
	errEmptyType       = ErrEmptyType
)

// Node is one architecture node. Spec is the sibling markdown, not a JSON field.
// Unknown holds JSON keys this version does not interpret, preserved on write.
type Node struct {
	ID             NodeID
	Name           string
	Type           string
	Status         Status
	ParentID       NodeID
	Implementation []string
	Scope          []string
	Protected      bool
	Spec           string
	Unknown        map[string]jsonRaw
}

// Relationship is a directed cross-cutting edge. It is not a hierarchy edge.
type Relationship struct {
	From    NodeID
	To      NodeID
	Label   string
	Kind    string
	Unknown map[string]jsonRaw
}

// Assignment is the machine-local active assignment in local.json.
type Assignment struct {
	NodeID     NodeID
	AssignedAt string
	Unknown    map[string]jsonRaw
}

// Severity distinguishes integrity warnings from load errors that still return an index.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Diagnostic is an integrity finding. Malformed JSON does not become a diagnostic;
// Open returns that as an error and no index.
type Diagnostic struct {
	Severity Severity
	Path     string
	Message  string
}

func (d Diagnostic) String() string {
	if d.Path == "" {
		return string(d.Severity) + ": " + d.Message
	}
	return string(d.Severity) + ": " + d.Path + ": " + d.Message
}

// CreateInput is the caller-supplied content for a new node. ID is derived.
type CreateInput struct {
	Name           string
	Type           string
	Spec           string
	ParentID       NodeID
	Implementation []string
	Scope          []string
	Protected      bool
	Status         Status
}

// UpdateInput patches a node. Nil pointers leave the field unchanged.
// Parent, when non-nil, sets parent_id; an empty NodeID clears it.
type UpdateInput struct {
	Name           *string
	Type           *string
	Spec           *string
	Status         *Status
	Parent         *NodeID
	Implementation *[]string
	Scope          *[]string
	Protected      *bool
}

// EffectiveScope returns scope, or implementation when scope is absent or empty.
// It does not rewrite the node.
func EffectiveScope(n *Node) []string {
	if n == nil {
		return nil
	}
	if len(n.Scope) > 0 {
		return append([]string{}, n.Scope...)
	}
	return append([]string{}, n.Implementation...)
}

func validStatus(s Status) bool {
	switch s {
	case StatusDraft, StatusSpecified, StatusAssigned, StatusInProgress, StatusDone, StatusBlocked:
		return true
	default:
		return false
	}
}

func validKind(k string) bool {
	switch k {
	case KindSync, KindAsync, KindData:
		return true
	default:
		return false
	}
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}

func cloneRaw(in map[string]jsonRaw) map[string]jsonRaw {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]jsonRaw, len(in))
	for k, v := range in {
		out[k] = append(jsonRaw(nil), v...)
	}
	return out
}

func statusList() string {
	return fmt.Sprintf("%s, %s, %s, %s, %s, %s",
		StatusDraft, StatusSpecified, StatusAssigned, StatusInProgress, StatusDone, StatusBlocked)
}
