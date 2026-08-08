package handlers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
	"net/http"

	"go.uber.org/zap"
)

type RoleHandler struct {
	Log         *zap.Logger
	RoleUsecase contracts.RoleUsecase
}

func NewRoleHandler(log *zap.Logger, roleUsecase contracts.RoleUsecase) *RoleHandler {
	return &RoleHandler{Log: log, RoleUsecase: roleUsecase}
}

func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.RoleUsecase.ListRoles(r.Context())
	if err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, roles)
}

type permissionRequest struct {
	Role   string `json:"role"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func (h *RoleHandler) AddPermission(w http.ResponseWriter, r *http.Request) {
	h.updatePermission(w, r, h.RoleUsecase.AddPermission)
}

func (h *RoleHandler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	h.updatePermission(w, r, h.RoleUsecase.RemovePermission)
}

// updatePermission decodes a permission request, applies the policy mutation,
// and writes the standard success response.
func (h *RoleHandler) updatePermission(w http.ResponseWriter, r *http.Request, apply func(context.Context, string, string, string) error) {
	var req permissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	if err := apply(r.Context(), req.Role, req.Method, req.Path); err != nil {
		utils.BuildErrorResponse(h.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, nil)
}
