package object

func NewSystem(systemID string) *Object {
	return &Object{
		ObjectID:   "public_policy",
		ObjectType: systemID,
	}
}

func NewSystemResource(systemID, resourceID string) *Object {
	return &Object{
		ObjectID:   resourceID,
		ObjectType: systemID,
	}
}
