package relation

type Relation string

const (
	MemberRelation         Relation = "member"
	CheckedMemberRelation  Relation = "checked_member"
	InternalMemberRelation Relation = "internal_member"
	HRMemberRelation       Relation = "hr_member"
	A4RoleMemberRelation   Relation = "a4_role_member"
	AllMembersRelation     Relation = "all_members"
)
