package controllers

import "github.com/konsulin-care/konsulin-api/internal/app/contracts"

type RoleController struct {
	RoleUsecase contracts.RoleUsecase
}

func NewRoleController(roleUsecase contracts.RoleUsecase) *RoleController {
	return &RoleController{
		RoleUsecase: roleUsecase,
	}
}
