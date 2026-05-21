package object

func NewGroup(groupID string) *Object {
	return &Object{
		ObjectID:   groupID,
		ObjectType: "group",
	}
}
