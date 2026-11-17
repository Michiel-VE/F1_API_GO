package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"f1-api-go/internal/models"
	"f1-api-go/internal/requests"
	"f1-api-go/internal/response"
	"f1-api-go/repository"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrAccessDenied = errors.New("access denied")
	ErrInvalidState = errors.New("invalid operation state")
)

const maxPoolNameLength = 60

// Repositories return (nil, nil) when nothing is found, so every call site
// below checks both the error and the nil result before using the value.

type PoolRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Pool, error)
	FindByUserId(ctx context.Context, userID uuid.UUID) ([]*models.Pool, error)
	FindByInviteCode(ctx context.Context, code string) (*models.Pool, error)
	Save(ctx context.Context, pool *models.Pool) (*models.Pool, error)
	AddMember(ctx context.Context, poolID, userID uuid.UUID) error
	RemoveMember(ctx context.Context, poolID, userID uuid.UUID) error
	Delete(ctx context.Context, pool *models.Pool) error
}

type TeamPredictionRepository interface {
	FindPersonalByUserId(ctx context.Context, userID uuid.UUID) (*models.TeamPrediction, error)
	FindByUserIdAndPoolId(ctx context.Context, userID, poolID uuid.UUID) (*models.TeamPrediction, error)
	FindByPoolId(ctx context.Context, poolID uuid.UUID) ([]*models.TeamPrediction, error)
	Create(ctx context.Context, prediction *models.TeamPrediction) error
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

func teamIDStrings(teams []models.Team) []string {
	ids := make([]string, len(teams))
	for i, team := range teams {
		ids[i] = team.ID.String()
	}
	return ids
}

// GetPersonalTeamPrediction returns the user's one personal prediction, or
// ErrNotFound when they have not made one yet (the frontend should treat that
// 404 as "show the empty prediction form").
func (s *PredictionService) GetPersonalTeamPrediction(ctx context.Context, userID uuid.UUID) (*response.SavedPredictionResponse, error) {
	prediction, err := s.teamPredictionRepo.FindPersonalByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	if prediction == nil {
		return nil, fmt.Errorf("%w: no personal team prediction found for this user", ErrNotFound)
	}

	return &response.SavedPredictionResponse{PredictedTeams: teamIDStrings(prediction.PredictedTeams)}, nil
}

func (s *PredictionService) GetPoolTeamPrediction(ctx context.Context, userID, poolID uuid.UUID) (*response.SavedPredictionResponse, error) {
	pool, err := s.poolRepo.FindByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("%w: pool not found", ErrNotFound)
	}

	if !pool.IsMember(userID) {
		return nil, fmt.Errorf("%w: you are not a member of this pool", ErrAccessDenied)
	}

	prediction, err := s.teamPredictionRepo.FindByUserIdAndPoolId(ctx, userID, poolID)
	if err != nil {
		return nil, err
	}
	if prediction == nil {
		return nil, fmt.Errorf("%w: no team prediction found for this pool", ErrNotFound)
	}

	return &response.SavedPredictionResponse{PredictedTeams: teamIDStrings(prediction.PredictedTeams)}, nil
}

// CreateTeamPrediction stores a prediction. Rules:
//   - no poolId   -> the user's personal prediction; a user can have only ONE
//   - with poolId -> a prediction inside that pool; one per user per pool, and
//     the user must be a member
//
// The "already exists" check below gives a friendly error; the unique indexes in
// db/prediction_constraints.sql make it impossible to slip past it with two
// simultaneous requests.
func (s *PredictionService) CreateTeamPrediction(ctx context.Context, userID uuid.UUID, req *requests.CreateSeasonPredictionRequest) error {
	seen := make(map[uuid.UUID]struct{}, len(req.PredictedTeams))
	for _, id := range req.PredictedTeams {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: team %s appears more than once", ErrInvalidState, id)
		}
		seen[id] = struct{}{}
	}

	alreadyExists := func() error {
		if req.PoolID == nil {
			return fmt.Errorf("%w: you already have a personal prediction", ErrInvalidState)
		}
		return fmt.Errorf("%w: you already submitted a prediction for this pool", ErrInvalidState)
	}

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var pool *models.Pool
		if req.PoolID != nil {
			var err error
			pool, err = s.poolRepo.FindByID(txCtx, *req.PoolID)
			if err != nil {
				return err
			}
			if pool == nil {
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
		if err != nil {
			return err
		}
		if existing != nil {
			return alreadyExists()
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
			team, ok := teamMap[id]
			if !ok || team == nil {
				return fmt.Errorf("%w: team %s does not exist", ErrNotFound, id)
			}
			orderedTeams = append(orderedTeams, *team)
		}

		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		now := time.Now()
		prediction := &models.TeamPrediction{
			User:           *user,
			Pool:           pool,
			PredictedTeams: orderedTeams,
		}
		prediction.ID = uuid.New()
		prediction.CreatedAt = now
		prediction.UpdatedAt = now

		if err := s.teamPredictionRepo.Create(txCtx, prediction); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return alreadyExists()
			}
			return err
		}
		return nil
	})
}

func (s *PredictionService) GetUserPools(ctx context.Context, userID uuid.UUID) ([]*response.PoolSummaryResponse, error) {
	pools, err := s.poolRepo.FindByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]*response.PoolSummaryResponse, 0, len(pools))
	for _, pool := range pools {
		if pool == nil {
			continue
		}
		creatorID := uuid.Nil
		if pool.Creator != nil {
			creatorID = pool.Creator.ID
		}

		responses = append(responses, &response.PoolSummaryResponse{
			ID:          pool.ID,
			Name:        pool.Name,
			MemberCount: len(pool.Members),
			CreatorID:   creatorID,
			InviteCode:  pool.InviteCode,
		})
	}

	return responses, nil
}

func (s *PredictionService) GetPoolDetails(ctx context.Context, userID, poolID uuid.UUID) (*response.PoolDetailsResponse, error) {
	pool, err := s.poolRepo.FindByID(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if pool == nil {
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
		if p != nil {
			predMap[p.User.ID] = p
		}
	}

	leaderBoard := make([]response.MemberPredictionDTO, 0, len(pool.Members))
	for _, member := range pool.Members {
		if member == nil {
			continue
		}

		predictedTeamIDs := []string{}
		if pred, ok := predMap[member.ID]; ok {
			predictedTeamIDs = teamIDStrings(pred.PredictedTeams)
		}

		userName := ""
		if member.Name != nil {
			userName = *member.Name
		}

		userPicture := ""
		if member.Picture != nil {
			userPicture = *member.Picture
		}

		leaderBoard = append(leaderBoard, response.MemberPredictionDTO{
			UserID:           member.ID,
			Username:         userName,
			Picture:          userPicture,
			PredictedTeamIDs: predictedTeamIDs,
		})
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

	personal, err := s.teamPredictionRepo.FindPersonalByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &response.PredictionStatusResponse{
		HasPools:              len(pools) > 0,
		HasPersonalPrediction: personal != nil,
	}, nil
}

const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateInviteCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate invite code: %w", err)
	}
	for i := range b {
		b[i] = inviteAlphabet[int(b[i])%len(inviteAlphabet)]
	}
	return string(b), nil
}

func (s *PredictionService) CreatePool(ctx context.Context, userID uuid.UUID, req *requests.CreatePoolRequest) (*models.Pool, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > maxPoolNameLength {
		return nil, fmt.Errorf("%w: pool name must be between 1 and %d characters", ErrInvalidState, maxPoolNameLength)
	}

	var created *models.Pool
	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		var inviteCode string
		for attempt := 0; attempt < 5; attempt++ {
			code, err := generateInviteCode()
			if err != nil {
				return err
			}
			clash, err := s.poolRepo.FindByInviteCode(txCtx, code)
			if err != nil {
				return err
			}
			if clash == nil {
				inviteCode = code
				break
			}
		}
		if inviteCode == "" {
			return errors.New("could not generate a unique invite code")
		}

		now := time.Now()
		pool := &models.Pool{
			Name:       name,
			InviteCode: inviteCode,
			CreatorID:  user.ID,
			Creator:    user,
		}
		pool.ID = uuid.New()
		pool.CreatedAt = now
		pool.UpdatedAt = now

		// 1. Save the pool record first without members to satisfy the foreign key
		saved, err := s.poolRepo.Save(txCtx, pool)
		if err != nil {
			return err
		}

		// 2. Explicitly add the creator as a member via the association table
		if err := s.poolRepo.AddMember(txCtx, saved.ID, user.ID); err != nil {
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
			return err
		}
		if pool == nil {
			return fmt.Errorf("%w: pool not found", ErrNotFound)
		}

		if !pool.IsMember(userID) {
			return fmt.Errorf("%w: you are not a member of this pool", ErrInvalidState)
		}

		isCreator := pool.Creator != nil && pool.Creator.ID == userID
		if isCreator || len(pool.Members) <= 1 {
			if err := s.teamPredictionRepo.DeleteByPoolId(txCtx, poolID); err != nil {
				return err
			}
			return s.poolRepo.Delete(txCtx, pool)
		}

		if err := s.poolRepo.RemoveMember(txCtx, poolID, userID); err != nil {
			return err
		}

		pred, err := s.teamPredictionRepo.FindByUserIdAndPoolId(txCtx, userID, poolID)
		if err != nil {
			return err
		}
		if pred != nil {
			return s.teamPredictionRepo.Delete(txCtx, pred)
		}
		return nil
	})
}

func (s *PredictionService) KickMemberOrDeletePool(ctx context.Context, adminID, poolID, memberID uuid.UUID) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		pool, err := s.poolRepo.FindByID(txCtx, poolID)
		if err != nil {
			return err
		}
		if pool == nil {
			return fmt.Errorf("%w: pool not found", ErrNotFound)
		}

		if pool.Creator == nil || pool.Creator.ID != adminID {
			return fmt.Errorf("%w: only the pool creator can remove members", ErrAccessDenied)
		}

		if adminID == memberID || len(pool.Members) <= 1 {
			if err := s.teamPredictionRepo.DeleteByPoolId(txCtx, poolID); err != nil {
				return err
			}
			return s.poolRepo.Delete(txCtx, pool)
		}

		if !pool.IsMember(memberID) {
			return fmt.Errorf("%w: target user is not a member of this pool", ErrNotFound)
		}

		if err := s.poolRepo.RemoveMember(txCtx, poolID, memberID); err != nil {
			return err
		}

		pred, err := s.teamPredictionRepo.FindByUserIdAndPoolId(txCtx, memberID, poolID)
		if err != nil {
			return err
		}
		if pred != nil {
			return s.teamPredictionRepo.Delete(txCtx, pred)
		}
		return nil
	})
}

func (s *PredictionService) JoinPool(ctx context.Context, userID uuid.UUID, req *requests.JoinPoolRequest) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		pool, err := s.poolRepo.FindByInviteCode(txCtx, strings.ToUpper(strings.TrimSpace(req.InviteCode)))
		if err != nil {
			return err
		}
		if pool == nil {
			return fmt.Errorf("%w: invalid invite code", ErrNotFound)
		}

		user, err := s.userRepo.FindByID(txCtx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		if pool.IsMember(userID) {
			return fmt.Errorf("%w: you are already a member of this pool", ErrInvalidState)
		}

		return s.poolRepo.AddMember(txCtx, pool.ID, userID)
	})
}

func (s *PredictionService) CreatePrediction(ctx context.Context, id uuid.UUID, req *requests.CreatePredictionRequest) error {
	return fmt.Errorf("%w: race predictions are not implemented yet", ErrInvalidState)
}
