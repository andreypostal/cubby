package dto

type CreateWorkspaceReq struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type UpdateWorkspaceReq struct {
	Name string `json:"name"     validate:"omitempty,min=2,max=100"`
}

// WorkspaceResp is the public representation of a workspace
type WorkspaceResp struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	// IsCreator tells whether the requester owns the workspace, and with it
	// whether they may share, rename or delete it
	IsCreator bool `json:"is_creator"`
}

type WorkspaceListResp struct {
	Workspaces []WorkspaceResp `json:"workspaces"`
}

// SwitchWorkspaceReq the active workspace switched and is stored as a claim in the refreshed JWT
type SwitchWorkspaceReq struct {
	WorkspaceID uint `json:"workspace_id" validate:"required,uint"`
}

// ShareWorkspaceReq grants a user access to the workspace by their username
type ShareWorkspaceReq struct {
	Username string `json:"username" validate:"required,min=1,max=255"`
}

// WorkspaceMemberResp is a user the workspace was shared with
type WorkspaceMemberResp struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type WorkspaceMemberListResp struct {
	Members []WorkspaceMemberResp `json:"members"`
}
