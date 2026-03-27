package onboarding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Session, error) {
	const query = `
		SELECT id, title, job_description, account_mode, objective, primary_platform, brand_name, audience, voice_summary, constraints, review_mode, approval_status, audit, activation, status, created_at, updated_at
		FROM onboarding_sessions
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list onboarding sessions: %w", err)
	}
	defer rows.Close()

	var items []Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		questions, err := r.ListQuestions(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		session.Questions = questions
		items = append(items, session)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateSession(ctx context.Context, session Session) error {
	const query = `
		INSERT INTO onboarding_sessions (id, title, job_description, account_mode, objective, primary_platform, brand_name, audience, voice_summary, constraints, review_mode, approval_status, audit, activation, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`
	auditPayload, err := json.Marshal(session.Audit)
	if err != nil {
		return fmt.Errorf("marshal onboarding audit: %w", err)
	}
	activationPayload, err := json.Marshal(session.Activation)
	if err != nil {
		return fmt.Errorf("marshal onboarding activation: %w", err)
	}
	_, err = r.pool.Exec(ctx, query, session.ID, session.Title, session.JobDescription, session.AccountMode, session.Objective, session.PrimaryPlatform, session.BrandName, session.Audience, session.VoiceSummary, session.Constraints, session.ReviewMode, session.ApprovalStatus, auditPayload, activationPayload, session.Status, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create onboarding session: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdateSession(ctx context.Context, session Session) error {
	const query = `
		UPDATE onboarding_sessions
		SET title=$2, job_description=$3, account_mode=$4, objective=$5, primary_platform=$6, brand_name=$7, audience=$8, voice_summary=$9, constraints=$10, review_mode=$11, approval_status=$12, audit=$13, activation=$14, status=$15, updated_at=$16
		WHERE id=$1
	`
	auditPayload, err := json.Marshal(session.Audit)
	if err != nil {
		return fmt.Errorf("marshal onboarding audit: %w", err)
	}
	activationPayload, err := json.Marshal(session.Activation)
	if err != nil {
		return fmt.Errorf("marshal onboarding activation: %w", err)
	}
	commandTag, err := r.pool.Exec(ctx, query, session.ID, session.Title, session.JobDescription, session.AccountMode, session.Objective, session.PrimaryPlatform, session.BrandName, session.Audience, session.VoiceSummary, session.Constraints, session.ReviewMode, session.ApprovalStatus, auditPayload, activationPayload, session.Status, session.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update onboarding session: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *PostgresRepository) GetSession(ctx context.Context, sessionID string) (Session, error) {
	const query = `
		SELECT id, title, job_description, account_mode, objective, primary_platform, brand_name, audience, voice_summary, constraints, review_mode, approval_status, audit, activation, status, created_at, updated_at
		FROM onboarding_sessions
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, sessionID)
	session, err := scanSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	session.Questions, err = r.ListQuestions(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (r *PostgresRepository) ListQuestions(ctx context.Context, sessionID string) ([]Question, error) {
	const query = `
		SELECT id, session_id, prompt, category, COALESCE(answer, ''), required, created_at, COALESCE(answered_at, TIMESTAMPTZ '0001-01-01')
		FROM onboarding_questions
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list onboarding questions: %w", err)
	}
	defer rows.Close()

	var items []Question
	for rows.Next() {
		var question Question
		if err := rows.Scan(&question.ID, &question.SessionID, &question.Prompt, &question.Category, &question.Answer, &question.Required, &question.CreatedAt, &question.AnsweredAt); err != nil {
			return nil, fmt.Errorf("scan onboarding question: %w", err)
		}
		items = append(items, question)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) UpsertQuestion(ctx context.Context, question Question) error {
	const query = `
		INSERT INTO onboarding_questions (id, session_id, prompt, category, answer, required, created_at, answered_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE
		SET prompt=EXCLUDED.prompt, category=EXCLUDED.category, answer=EXCLUDED.answer, required=EXCLUDED.required, answered_at=EXCLUDED.answered_at
	`
	_, err := r.pool.Exec(ctx, query, question.ID, question.SessionID, question.Prompt, question.Category, nullString(question.Answer), question.Required, question.CreatedAt, nullTime(question.AnsweredAt))
	if err != nil {
		return fmt.Errorf("upsert onboarding question: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetQuestion(ctx context.Context, sessionID, questionID string) (Question, error) {
	const query = `
		SELECT id, session_id, prompt, category, COALESCE(answer, ''), required, created_at, COALESCE(answered_at, TIMESTAMPTZ '0001-01-01')
		FROM onboarding_questions
		WHERE session_id = $1 AND id = $2
	`
	var question Question
	err := r.pool.QueryRow(ctx, query, sessionID, questionID).Scan(&question.ID, &question.SessionID, &question.Prompt, &question.Category, &question.Answer, &question.Required, &question.CreatedAt, &question.AnsweredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Question{}, ErrQuestionNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("get onboarding question: %w", err)
	}
	return question, nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner) (Session, error) {
	var session Session
	var auditPayload []byte
	var activationPayload []byte
	err := scanner.Scan(&session.ID, &session.Title, &session.JobDescription, &session.AccountMode, &session.Objective, &session.PrimaryPlatform, &session.BrandName, &session.Audience, &session.VoiceSummary, &session.Constraints, &session.ReviewMode, &session.ApprovalStatus, &auditPayload, &activationPayload, &session.Status, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return Session{}, err
	}
	if len(auditPayload) > 0 {
		if err := json.Unmarshal(auditPayload, &session.Audit); err != nil {
			return Session{}, fmt.Errorf("unmarshal onboarding audit: %w", err)
		}
	}
	if len(activationPayload) > 0 {
		if err := json.Unmarshal(activationPayload, &session.Activation); err != nil {
			return Session{}, fmt.Errorf("unmarshal onboarding activation: %w", err)
		}
	}
	return session, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value any) any {
	switch typed := value.(type) {
	case interface{ IsZero() bool }:
		if typed.IsZero() {
			return nil
		}
	}
	return value
}
