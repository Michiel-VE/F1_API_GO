package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"f1-api-go/internal/models"
	"f1-api-go/internal/requests"
	"f1-api-go/internal/response"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrAccessDenied = errors.New("access denied")
	ErrInvalidState = errors.New("invalid operation state")
)

type PoolRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Pool, error)
	FindByUserId(ctx context.Context, userID uuid.UUID) ([]*models.Pool, error)
	FindByInviteCode(ctx context.Context, code string) (*models.Pool, error)
	Save(ctx context.Context, pool *models.Pool) (*models.Pool, error)
	Delete(ctx context.Context, pool *models.Pool) error
}

type TeamPredictionRepository interface {
	FindPersonalByUserId(ctx context.Context, userID uuid.UUID) (*models.TeamPrediction, error)
	FindByUserIdAndPoolId(ctx context.Context, userID, poolID uuid.UUID) (*models.TeamPrediction, error)
	FindByPoolId(ctx context.Context, poolID uuid.UUID) ([]*models.TeamPrediction, error)
	Save(ctx context.Context, prediction *models.TeamPrediction) error
	Delete(ctx context.Context, prediction *models.TeamPrediction) error
	DeleteByPoolId(ctx context.Context, poolID uuid.UUID) error
}

type PredictionTeamRepository interface {
	FindAllById(ctx context.Context, ids []uuid.UUID) ([]*models.Team, error)
}

type PredictionUserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type TransactionManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type PredictionService struct {
	poolRepo           PoolRepository
	teamPredictionRepo TeamPredictionRepository
	teamRepo           PredictionTeamRepository
	userRepo           PredictionUserRepository
	txManager          TransactionManager
}

func NewPredictionService(
	poolRepo PoolRepository,
	teamPredictionRepo TeamPredictionRepository,
	teamRepo PredictionTeamRepository,
	userRepo PredictionUserRepository,
	txManager TransactionManager,
) *PredictionService {
	return &PredictionService{
		poolRepo:           poolRepo,
		teamPredictionRepo: teamPredictionRepo,
		teamRepo:           teamRepo,
		userRepo:           userRepo,
		txManager:          txManager,
	}
}

func (s *PredictionService) GetPersonalTeamPrediction(ctx context.Context, userID uuid.UUID) (*response.SavedPredictionResponse, error) {
	prediction, err := s.teamPredictionRepo.FindPersonalByUserId(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: no personal team prediction found for this user", ErrNotFound)
	}

	teamIDs := make([]string, len(prediction.PredictedTeams))
	for i, team := range prediction.PredictedTeams {
		teamIDs[i] = team.ID.String()
	}

	return &response.SavedPredictionResponse{PredictedTeams: teamIDs}, nil
}

func (s *PredictionService) GetPoolTeamPrediction(ctx context.Context, userID, poolID uuid.UUID) (*response.SavedPredictionResponse, error) {
	pool, err := s.poolRepo.FindByID(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("%w: pool not found", ErrNotFound)
	}

	if !pool.IsMember(userID) {
		return nil, fmt.Errorf("%w: you are not a member of this pool", ErrAccessDenied)
	}

	prediction, err := s.teamPredictionRepo.FindByUserIdAndPoolId(ctx, userID, poolID)
	if err != nil {
		return nil, fmt.Errorf("%w: no team prediction found for this pool", ErrNotFound)
	}

	teamIDs := make([]string, len(prediction.PredictedTeams))
	for i, team := range prediction.PredictedTeams {
		teamIDs[i] = team.ID.String()
	}

	return &response.SavedPredictionResponse{PredictedTeams: teamIDs}, nil
}

func (s *PredictionService) CreateTeamPrediction(ctx context.Context, userID uuid.UUID, req *requests.CreateSeasonPredictionRequest) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var pool *models.Pool
		if req.PoolID != nil {
			var err error
			pool, err = s.poolRepo.FindByID(txCtx, *req.PoolID)
			if err != nil {
				return fmt.Errorf("%w: pool not found", ErrNotFound)
			}
			if !pool.IsMember(userID) {
				return fmt.Errorf("%w: you are not part of this pool", ErrAccessDenied)
			}
		}

		var (
			existing *models.TeamPrediction
			err      error
		)
		if req.PoolID == nil {
			existing, err = s.teamPredictionRepo.FindPersonalByUserId(txCtx, userID)
		} else {
			existing, err = s.teamPredictionRepo.FindByUserIdAndPoolId(txCtx, userID, *req.PoolID)
		}

		if err == nil && existing != nil {
			return fmt.Errorf("%w: you have already submitted a prediction for this context", ErrInvalidState)
		}

		fetchedTeams, err := s.teamRepo.FindAllById(txCtx, req.PredictedTeams)
		if err != nil {
			return err
		}

		teamMap := make(map[uuid.UUID]*models.Team, len(fetchedTeams))
		for _, t := range fetchedTeams {
			teamMap[t.ID] = t
		}

		orderedTeams := make([]models.Team, 0, len(req.PredictedTeams))
		for _, id := range req.PredictedTeams {
			if team, ok := teamMap[id]; ok && team != nil {
				orderedTeams = append(orderedTeams, *team)
			}
		}

		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil || user == nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		now := time.Now()
		prediction := &models.TeamPrediction{
			ID:             uuid.New(),
			User:           *user,
			Pool:           pool,
			PredictedTeams: orderedTeams,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		return s.teamPredictionRepo.Save(txCtx, prediction)
	})
}

func (s *PredictionService) GetUserPools(ctx context.Context, userID uuid.UUID) ([]*response.PoolSummaryResponse, error) {
	pools, err := s.poolRepo.FindByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]*response.PoolSummaryResponse, len(pools))
	for i, pool := range pools {
		creatorID := uuid.Nil
		if pool.Creator != nil {
			creatorID = pool.Creator.ID
		}

		responses[i] = &response.PoolSummaryResponse{
			ID:          pool.ID,
			Name:        pool.Name,
			MemberCount: len(pool.Members),
			CreatorID:   creatorID,
			InviteCode:  pool.InviteCode,
		}
	}

	return responses, nil
}

func (s *PredictionService) GetPoolDetails(ctx context.Context, userID, poolID uuid.UUID) (*response.PoolDetailsResponse, error) {
	pool, err := s.poolRepo.FindByID(ctx, poolID)
	if err != nil {
		return nil, fmt.Errorf("%w: pool not found", ErrNotFound)
	}

	if !pool.IsMember(userID) {
		return nil, fmt.Errorf("%w: access denied to this pool dashboard", ErrAccessDenied)
	}

	predictions, err := s.teamPredictionRepo.FindByPoolId(ctx, poolID)
	if err != nil {
		return nil, err
	}

	predMap := make(map[uuid.UUID]*models.TeamPrediction, len(predictions))
	for _, p := range predictions {
		predMap[p.User.ID] = p
	}

	leaderBoard := make([]response.MemberPredictionDTO, len(pool.Members))
	for i, member := range pool.Members {
		if member == nil {
			continue
		}

		var predictedTeamIDs []string
		if pred, ok := predMap[member.ID]; ok {
			predictedTeamIDs = make([]string, len(pred.PredictedTeams))
			for j, team := range pred.PredictedTeams {
				predictedTeamIDs[j] = team.ID.String()
			}
		} else {
			predictedTeamIDs = []string{}
		}

		userName := ""
		if member.Name != nil {
			userName = *member.Name
		}

		userPicture := ""
		if member.Picture != nil {
			userPicture = *member.Picture
		}

		leaderBoard[i] = response.MemberPredictionDTO{
			UserID:           member.ID,
			Username:         userName,
			Picture:          userPicture,
			PredictedTeamIDs: predictedTeamIDs,
		}
	}

	creatorID := uuid.Nil
	if pool.Creator != nil {
		creatorID = pool.Creator.ID
	}

	return &response.PoolDetailsResponse{
		PoolID:      pool.ID,
		PoolName:    pool.Name,
		CreatorID:   creatorID,
		LeaderBoard: leaderBoard,
	}, nil
}

func (s *PredictionService) GetPredictionStatus(ctx context.Context, userID uuid.UUID) (*response.PredictionStatusResponse, error) {
	pools, err := s.poolRepo.FindByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}

	personal, _ := s.teamPredictionRepo.FindPersonalByUserId(ctx, userID)

	return &response.PredictionStatusResponse{
		HasPools:              len(pools) > 0,
		HasPersonalPrediction: personal != nil,
	}, nil
}

func (s *PredictionService) CreatePool(ctx context.Context, userID uuid.UUID, req *requests.CreatePoolRequest) (*models.Pool, error) {
	var created *models.Pool
	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		inviteCode := strings.ToUpper(uuid.New().String()[:8])
		now := time.Now()

		pool := &models.Pool{
			ID:         uuid.New(),
			Name:       req.Name,
			InviteCode: inviteCode,
			Creator:    user,
			Members:    []*models.User{user},
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		saved, err := s.poolRepo.Save(txCtx, pool)
		if err != nil {
			return err
		}
		created = saved
		return nil
	})

	return created, err
}

func (s *PredictionService) LeaveOrDeletePool(ctx context.Context, userID, poolID uuid.UUID) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		pool, err := s.poolRepo.FindByID(txCtx, poolID)
		if err != nil {
			return fmt.Errorf("%w: pool not found", ErrNotFound)
		}

		if !pool.IsMember(userID) {
			return fmt.Errorf("%w: you are not a member of this pool", ErrInvalidState)
		}

		if pool.Creator != nil && pool.Creator.ID == userID || len(pool.Members) <= 1 {
			if err := s.teamPredictionRepo.DeleteByPoolId(txCtx, poolID); err != nil {
				return err
			}
			return s.poolRepo.Delete(txCtx, pool)
		}

		pool.RemoveMember(userID)

		if pred, err := s.teamPredictionRepo.FindByUserIdAndPoolId(txCtx, userID, poolID); err == nil && pred != nil {
			if err := s.teamPredictionRepo.Delete(txCtx, pred); err != nil {
				return err
			}
		}

		_, err = s.poolRepo.Save(txCtx, pool)
		return err
	})
}

func (s *PredictionService) KickMemberOrDeletePool(ctx context.Context, adminID, poolID, memberID uuid.UUID) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		pool, err := s.poolRepo.FindByID(txCtx, poolID)
		if err != nil {
			return fmt.Errorf("%w: pool not found", ErrNotFound)
		}

		if pool.Creator != nil && pool.Creator.ID != adminID {
			return fmt.Errorf("%w: only the pool creator can remove members", ErrAccessDenied)
		}

		if adminID == memberID || len(pool.Members) <= 1 {
			if err := s.teamPredictionRepo.DeleteByPoolId(txCtx, poolID); err != nil {
				return err
			}
			return s.poolRepo.Delete(txCtx, pool)
		}

		if !pool.RemoveMember(memberID) {
			return fmt.Errorf("%w: target user is not a member of this pool", ErrNotFound)
		}

		if pred, err := s.teamPredictionRepo.FindByUserIdAndPoolId(txCtx, memberID, poolID); err == nil && pred != nil {
			if err := s.teamPredictionRepo.Delete(txCtx, pred); err != nil {
				return err
			}
		}

		_, err = s.poolRepo.Save(txCtx, pool)
		return err
	})
}

func (s *PredictionService) JoinPool(ctx context.Context, userID uuid.UUID, req *requests.JoinPoolRequest) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		pool, err := s.poolRepo.FindByInviteCode(txCtx, strings.ToUpper(req.InviteCode))
		if err != nil {
			return fmt.Errorf("%w: invalid invite code", ErrNotFound)
		}

		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		if pool.IsMember(userID) {
			return fmt.Errorf("%w: you are already a member of this pool", ErrInvalidState)
		}

		pool.Members = append(pool.Members, user)
		_, err = s.poolRepo.Save(txCtx, pool)
		return err
	})
}

func (s *PredictionService) CreatePrediction(ctx context.Context, id uuid.UUID, req *requests.CreatePredictionRequest) error {
	return nil
}