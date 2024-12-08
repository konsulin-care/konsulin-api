package users

import (
	"context"
	"database/sql"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"

	"github.com/lib/pq"
)

type userPostgresRepository struct {
	DB *sql.DB
}

func NewUserPostgresRepository(db *sql.DB) contracts.UserRepository {
	return &userPostgresRepository{DB: db}
}

func (repo *userPostgresRepository) GetClient(ctx context.Context) interface{} {
	return repo.DB.Ping()
}

func (r *userPostgresRepository) CreateUser(ctx context.Context, user *models.User) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx, queries.CreateUserQuery,
		user.Email, user.Gender, user.RoleID, user.Address, user.Fullname,
		user.Username, user.Password, user.BirthDate, user.PatientID,
		user.ResetToken, user.WhatsAppOTP, user.WhatsAppNumber,
		user.PractitionerID, user.ProfilePictureName, pq.Array(user.Educations),
		user.ResetTokenExpiry, user.WhatsAppOTPExpiry,
	).Scan(&id)
	if err != nil {
		return "", exceptions.ErrPostgresDBInsertData(err)
	}
	return id, nil
}

func (r *userPostgresRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.findOne(ctx, "email", email)
}

func (r *userPostgresRepository) FindByWhatsAppNumber(ctx context.Context, whatsappNumber string) (*models.User, error) {
	return r.findOne(ctx, "whatsapp_number", whatsappNumber)
}

func (r *userPostgresRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.findOne(ctx, "username", username)
}

func (r *userPostgresRepository) FindByEmailOrUsername(ctx context.Context, email, username string) (*models.User, error) {
	return r.findUser(ctx, queries.FindByEmailOrUsernameQuery, email, username)
}

func (r *userPostgresRepository) FindByID(ctx context.Context, userID string) (*models.User, error) {
	return r.findOne(ctx, "id", userID)
}

func (r *userPostgresRepository) FindByResetToken(ctx context.Context, resetToken string) (*models.User, error) {
	return r.findOne(ctx, "reset_token", resetToken)
}

func (r *userPostgresRepository) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := r.DB.ExecContext(ctx, queries.UpdateUserQuery,
		user.Email, user.Gender, user.RoleID, user.Address, user.Fullname,
		user.Username, user.Password, user.BirthDate, user.PatientID,
		user.ResetToken, user.WhatsAppOTP, user.WhatsAppNumber,
		user.PractitionerID, user.ProfilePictureName, pq.Array(user.Educations),
		user.ResetTokenExpiry, user.WhatsAppOTPExpiry, user.ID,
	)
	if err != nil {
		return exceptions.ErrPostgresDBUpdateData(err)
	}
	return nil
}

func (r *userPostgresRepository) DeleteByID(ctx context.Context, userID string) error {
	_, err := r.DB.ExecContext(ctx, queries.DeleteByIDQuery, userID)
	if err != nil {
		return exceptions.ErrPostgresDBDeleteData(err)
	}
	return nil
}

func (r *userPostgresRepository) findOne(ctx context.Context, field string, value interface{}) (*models.User, error) {
	query := fmt.Sprintf(queries.FindByFieldQueryTemplate, field)
	return r.findUser(ctx, query, value)
}

func (r *userPostgresRepository) findUser(ctx context.Context, query string, args ...interface{}) (*models.User, error) {
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
			return nil, nil
		}
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

	return &user, nil
}
