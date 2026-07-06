package services

import (
	"time"

	"github.com/routerarchitects/mango-mdu-service/internal/gateway/prov"
	"github.com/routerarchitects/mango-mdu-service/internal/models"
)

type OperatorService struct {
	provClient *prov.Client
}

func NewOperatorService(provClient *prov.Client) *OperatorService {
	return &OperatorService{
		provClient: provClient,
	}
}

func (s *OperatorService) GetOperator(reqCtx prov.RequestContext, operatorID string) (*models.OperatorDetail, error) {
	provOp, err := s.provClient.GetOperator(reqCtx, operatorID)
	if err != nil {
		return nil, err
	}
	return mapProvToOperatorDetail(provOp), nil
}

func (s *OperatorService) UpdateOperator(reqCtx prov.RequestContext, operatorID string, req *models.UpdateOperatorRequest) (*models.OperatorDetail, error) {
	// 1. Fetch current operator to merge fields safely
	currOp, err := s.provClient.GetOperator(reqCtx, operatorID)
	if err != nil {
		return nil, err
	}

	// 2. Apply modifications
	if req.Name != nil {
		currOp.Name = *req.Name
	}
	if req.Description != nil {
		currOp.Description = *req.Description
	}
	if req.RegistrationID != nil {
		currOp.RegistrationID = *req.RegistrationID
	}

	// 3. Save modifications
	updatedOp, err := s.provClient.UpdateOperator(reqCtx, operatorID, currOp)
	if err != nil {
		return nil, err
	}

	return mapProvToOperatorDetail(updatedOp), nil
}

func (s *OperatorService) DeleteOperator(reqCtx prov.RequestContext, operatorID string) error {
	return s.provClient.DeleteOperator(reqCtx, operatorID)
}

func mapProvToOperatorDetail(provOp *prov.ProvOperator) *models.OperatorDetail {
	if provOp == nil {
		return nil
	}
	return &models.OperatorDetail{
		ID:             provOp.ID,
		Name:           provOp.Name,
		Description:    provOp.Description,
		EntityID:       provOp.EntityID,
		RegistrationID: provOp.RegistrationID,
		CreatedAt:      time.Unix(provOp.Created, 0).UTC(),
		UpdatedAt:      time.Unix(provOp.Modified, 0).UTC(),
	}
}
