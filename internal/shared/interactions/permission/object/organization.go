package object

func NewOrganization(organizationID string) *Object {
	return &Object{
		ObjectID:   organizationID,
		ObjectType: "organization",
	}
}
