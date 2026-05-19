package relationship

import (
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/caveat"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/object"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/relation"
	"github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/api/subject"
)

func NewGroupToSystemRelationship(rel relation.Relation, systemID, groupID string, options ...Option) *Relationship {
	return New(
		rel,
		*object.NewSystem(systemID),
		*subject.New(
			*object.NewGroup(groupID),
			subject.WithRelation(relation.MemberRelation),
		),
		options...,
	)
}

func NewGroupToSystemResourceRelationship(rel relation.Relation, systemID, resourceID, groupID string, options ...Option) *Relationship {
	return New(
		rel,
		*object.NewSystemResource(systemID, resourceID),
		*subject.New(
			*object.NewGroup(groupID),
			subject.WithRelation(relation.MemberRelation),
		),
		options...,
	)
}

func NewOrganizationToGroupRelationship(groupID, organizationID string, options ...Option) *Relationship {
	return New(
		relation.HRMemberRelation,
		*object.NewGroup(groupID),
		*subject.New(
			*object.NewOrganization(organizationID),
			subject.WithRelation(relation.MemberRelation),
		),
		options...,
	)
}

func NewAllEmployeeToGroupForA4Relationship(groupID string, options ...Option) *Relationship {
	return New(
		relation.A4RoleMemberRelation,
		*object.NewGroup(groupID),
		*subject.New(
			*object.NewEmployee("*"),
		),
		options...,
	)
}

func NewAllEmployeeToGroupForHRRelationship(groupID string, options ...Option) *Relationship {
	return New(
		relation.HRMemberRelation,
		*object.NewGroup(groupID),
		*subject.New(
			*object.NewEmployee("*"),
		),
		options...,
	)
}

func NewGroupWithStaticAttributesRelationship(groupID string, options ...caveat.StaticAttributesCheckOption) *Relationship {
	groupObj := object.NewGroup(groupID)
	return New(
		relation.CheckedMemberRelation,
		*groupObj,
		*subject.New(
			*groupObj,
			subject.WithRelation(relation.InternalMemberRelation),
		),
		WithCaveat(*caveat.NewStaticAttributesCheck(options...)),
	)
}

func NewA4RoleToGroupRelationship(groupID, a4Role string, options ...Option) *Relationship {
	return New(
		relation.A4RoleMemberRelation,
		*object.NewGroup(groupID),
		*subject.New(
			*object.NewA4Role(a4Role),
			subject.WithRelation(relation.AllMembersRelation),
		),
		options...,
	)
}
