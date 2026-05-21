package subject

import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relation"
)

type Subject struct {
	Object   object.Object      `json:"object"`
	Relation *relation.Relation `json:"optionalRelation,omitempty"`
}

type Option func(*Subject)

func WithRelation(rel relation.Relation) Option {
	return func(s *Subject) {
		s.Relation = &rel
	}
}

func New(obj object.Object, options ...Option) *Subject {
	sub := &Subject{
		Object: obj,
	}

	for _, opt := range options {
		opt(sub)
	}

	return sub
}
