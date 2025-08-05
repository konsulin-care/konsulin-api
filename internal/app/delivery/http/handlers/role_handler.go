package handlers

import (
	"encoding/json"
	"net/http"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"

	"go.uber.org/zap"
)

// RoleHandler provides admin endpoints for managing role permissions.
type RoleHandler struct {
	Log         *zap.Logger
	RoleUsecase contracts.RoleUsecase
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(log *zap.Logger, roleUsecase contracts.RoleUsecase) *RoleHandler {
	return &RoleHandler{Log: log, RoleUsecase: roleUsecase}
}

// ListRoles returns all roles known by the enforcer.
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.RoleUsecase.ListRoles(r.Context())
	if err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, roles)
}

// permissionRequest represents the payload for modifying permissions.
type permissionRequest struct {
	Role   string `json:"role"`
	Object string `json:"object"`
	Action string `json:"action"`
}

// AddPermission adds a permission to a role and reloads the policy.
func (h *RoleHandler) AddPermission(w http.ResponseWriter, r *http.Request) {
	var req permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	if err := h.RoleUsecase.AddPermission(r.Context(), req.Role, req.Object, req.Action); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, nil)
}

// RemovePermission removes a permission from a role and reloads the policy.
func (h *RoleHandler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	var req permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	if err := h.RoleUsecase.RemovePermission(r.Context(), req.Role, req.Object, req.Action); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, nil)
}
