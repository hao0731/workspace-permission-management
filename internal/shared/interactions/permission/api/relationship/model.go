package relationship

import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/caveat"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/relation"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/subject"
)

type Relationship struct {
	Relation relation.Relation `json:"relation"`
	Resource object.Object     `json:"resource"`
	Subject  subject.Subject   `json:"subject"`
	Caveat   *caveat.Caveat    `json:"optionalCaveat,omitempty"`
}

type Option func(*Relationship)

func WithCaveat(cav caveat.Caveat) Option {
	return func(r *Relationship) {
		r.Caveat = &cav
	}
}

func New(rel relation.Relation, res object.Object, sub subject.Subject, options ...Option) *Relationship {
	r := &Relationship{
		Relation: rel,
		Resource: res,
		Subject:  sub,
	}
	for _, opt := range options {
		opt(r)
	}
	return r
}
