package transport

import domainhr "github.com/hao0731/workspace-permission-management/internal/domain/hr"

type UserResponse struct {
	User UserDTO `json:"user"`
}

type UserListResponse struct {
	Users []UserDTO `json:"users"`
}

type UserDTO struct {
	NTAccount   string `json:"nt_account"`
	DisplayName string `json:"display_name"`
}

func NewUserResponse(user domainhr.User) UserResponse {
	return UserResponse{User: newUserDTO(user)}
}

func NewUserListResponse(users []domainhr.User) UserListResponse {
	response := UserListResponse{Users: make([]UserDTO, 0, len(users))}
	for _, user := range users {
		response.Users = append(response.Users, newUserDTO(user))
	}
	return response
}

func newUserDTO(user domainhr.User) UserDTO {
	user = user.Normalize()
	return UserDTO{
		NTAccount:   user.NTAccount,
		DisplayName: user.DisplayName,
	}
}
