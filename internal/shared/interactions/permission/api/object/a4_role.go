package object

func NewA4Role(objectID string) *Object {
	return &Object{
		ObjectID:   objectID,
		ObjectType: "business_role",
	}
}
