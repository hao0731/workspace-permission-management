package object

func NewEmployee(employeeID string) *Object {
	return &Object{
		ObjectID:   employeeID,
		ObjectType: "employee",
	}
}
