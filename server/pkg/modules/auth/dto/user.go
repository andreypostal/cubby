package dto

type LoginReq struct {
	Username string `json:"username" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse is returned after a successful login.
type AuthResponse struct {
	Token string      `json:"token"`
	User  UserPayload `json:"user"`
}

// UserPayload is the public representation of a user
type UserPayload struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Username      string `json:"username"`
	IsSystemAdmin bool   `json:"is_system_admin"`
}

type MeResponse struct {
	User             UserPayload    `json:"user"`
	CurrentWorkspace *WorkspaceResp `json:"current_workspace,omitempty"`
}

type UserListResp struct {
	Users []UserPayload `json:"users"`
}

type CreateUserReq struct {
	Name          string `json:"name" validate:"required,min=2,max=100"`
	Username      string `json:"username" validate:"required,min=2,max=100"`
	Password      string `json:"password" validate:"required,min=8,max=100"`
	IsSystemAdmin bool   `json:"is_system_admin"`
}

// UpdateUserReq carries everything an administrator may change about somebody else
type UpdateUserReq struct {
	Name          string `json:"name" validate:"required,min=2,max=100"`
	IsSystemAdmin bool   `json:"is_system_admin"`
}

type ChangePasswordReq struct {
	Password string `json:"password" validate:"required,min=8,max=100"`
}
