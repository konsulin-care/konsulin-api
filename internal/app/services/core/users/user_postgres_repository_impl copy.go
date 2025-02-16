package users

import (
	"context"
	"database/sql"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
	"sync"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

type userPostgresRepository struct {
	DB  *sql.DB
	Log *zap.Logger
}

var (
	userPostgresRepositoryInstance contracts.UserRepository
	onceUserPostgresRepository     sync.Once
)

func NewUserPostgresRepository(db *sql.DB, logger *zap.Logger) contracts.UserRepository {
	onceUserPostgresRepository.Do(func() {
		instance := &userPostgresRepository{
			DB:  db,
			Log: logger,
		}
		userPostgresRepositoryInstance = instance
	})
	return userPostgresRepositoryInstance
}

func (repo *userPostgresRepository) GetClient(ctx context.Context) interface{} {
	return repo.DB.Ping()
}
func (r *userPostgresRepository) CreateUser(ctx context.Context, user *models.User) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.CreateUser called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var id string
	err := r.DB.QueryRowContext(ctx, queries.CreateUserQuery,
		user.Email, user.Gender, user.RoleID, user.Address, user.Fullname,
		user.Username, user.Password, user.BirthDate, user.PatientID,
		user.ResetToken, user.WhatsAppOTP, user.WhatsAppNumber,
		user.PractitionerID, user.ProfilePictureName, pq.Array(user.Educations),
		user.ResetTokenExpiry, user.WhatsAppOTPExpiry,
	).Scan(&id)
	if err != nil {
		r.Log.Error("userPostgresRepository.CreateUser error inserting user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return "", exceptions.ErrPostgresDBInsertData(err)
	}

	r.Log.Info("userPostgresRepository.CreateUser succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, id),
	)
	return id, nil
}

func (r *userPostgresRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByEmail called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return r.findOne(ctx, "email", email)
}

func (r *userPostgresRepository) FindByWhatsAppNumber(ctx context.Context, whatsappNumber string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByWhatsAppNumber called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return r.findOne(ctx, "whatsapp_number", whatsappNumber)
}

func (r *userPostgresRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByUsername called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return r.findOne(ctx, "username", username)
}

func (r *userPostgresRepository) FindByEmailOrUsername(ctx context.Context, email, username string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByEmailOrUsername called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return r.findUser(ctx, queries.FindByEmailOrUsernameQuery, email, username)
}

func (r *userPostgresRepository) FindByID(ctx context.Context, userID string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, userID),
	)
	return r.findOne(ctx, "id", userID)
}

func (r *userPostgresRepository) FindByResetToken(ctx context.Context, resetToken string) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.FindByResetToken called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return r.findOne(ctx, "reset_token", resetToken)
}

func (r *userPostgresRepository) UpdateUser(ctx context.Context, user *models.User) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.UpdateUser called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)
	_, err := r.DB.ExecContext(ctx, queries.UpdateUserQuery,
		user.Email, user.Gender, user.RoleID, user.Address, user.Fullname,
		user.Username, user.Password, user.BirthDate, user.PatientID,
		user.ResetToken, user.WhatsAppOTP, user.WhatsAppNumber,
		user.PractitionerID, user.ProfilePictureName, pq.Array(user.Educations),
		user.ResetTokenExpiry, user.WhatsAppOTPExpiry, user.ID,
	)
	if err != nil {
		r.Log.Error("userPostgresRepository.UpdateUser error updating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, user.ID),
			zap.Error(err),
		)
		return exceptions.ErrPostgresDBUpdateData(err)
	}
	r.Log.Info("userPostgresRepository.UpdateUser succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)
	return nil
}

func (r *userPostgresRepository) DeleteByID(ctx context.Context, userID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.DeleteByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, userID),
	)
	_, err := r.DB.ExecContext(ctx, queries.DeleteByIDQuery, userID)
	if err != nil {
		r.Log.Error("userPostgresRepository.DeleteByID error deleting user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, userID),
			zap.Error(err),
		)
		return exceptions.ErrPostgresDBDeleteData(err)
	}
	r.Log.Info("userPostgresRepository.DeleteByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, userID),
	)
	return nil
}

func (r *userPostgresRepository) findOne(ctx context.Context, field string, value interface{}) (*models.User, error) {
	query := fmt.Sprintf(queries.FindByFieldQueryTemplate, field)
	return r.findUser(ctx, query, value)
}

func (r *userPostgresRepository) findUser(ctx context.Context, query string, args ...interface{}) (*models.User, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("userPostgresRepository.findUser called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	row := r.DB.QueryRowContext(ctx, query, args...)

	var user models.User
	var educations pq.StringArray
	var resetTokenExpiry, whatsappOTPExpiry sql.NullTime
	var deletedAt sql.NullTime

	err := row.Scan(
		&user.ID, &user.Email, &user.Gender, &user.RoleID, &user.Address, &user.Fullname,
		&user.Username, &user.Password, &user.BirthDate, &user.PatientID, &user.ResetToken,
		&user.WhatsAppOTP, &user.WhatsAppNumber, &user.PractitionerID, &user.ProfilePictureName,
		&educations, &resetTokenExpiry, &whatsappOTPExpiry, &user.CreatedAt, &user.UpdatedAt, &deletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			r.Log.Warn("userPostgresRepository.findUser no rows found",
				zap.String(constvars.LoggingRequestIDKey, requestID),
			)
			return nil, nil
		}
		r.Log.Error("userPostgresRepository.findUser error scanning row",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	user.Educations = educations
	if resetTokenExpiry.Valid {
		user.ResetTokenExpiry = &resetTokenExpiry.Time
	}
	if whatsappOTPExpiry.Valid {
		user.WhatsAppOTPExpiry = &whatsappOTPExpiry.Time
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}

	r.Log.Info("userPostgresRepository.findUser succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)
	return &user, nil
}
