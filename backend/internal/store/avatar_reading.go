package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

var (
	ErrReadingNotFound      = errors.New("store: avatar reading not found")
	ErrReadingNotReady      = errors.New("store: avatar reading not ready")
	ErrInvalidReadingAnswer = errors.New("store: invalid reading answer")
	ErrReadingInProgress    = errors.New("store: spirit animal reading already in progress")
)

const avatarReadingColumns = `
	id, user_id, status, draw, questions_json, user_answers, personality_json,
	totems_json, ranking_json, selected_totem_name, art_direction_version,
	error_message, created_at, updated_at, phase_started_at, completed_at
`

func scanAvatarReading(row interface{ Scan(dest ...any) error }) (*AvatarReading, error) {
	var r AvatarReading
	var draw pq.Int32Array
	var answers pq.StringArray
	var questions, personality, totems, ranking sql.NullString
	var selectedTotem, errMsg sql.NullString
	var phaseStartedAt sql.NullTime
	var completedAt sql.NullTime

	err := row.Scan(
		&r.ID,
		&r.UserID,
		&r.Status,
		&draw,
		&questions,
		&answers,
		&personality,
		&totems,
		&ranking,
		&selectedTotem,
		&r.ArtDirectionVersion,
		&errMsg,
		&r.CreatedAt,
		&r.UpdatedAt,
		&phaseStartedAt,
		&completedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrReadingNotFound
		}
		return nil, err
	}
	r.Draw = []int32(draw)
	if questions.Valid {
		r.QuestionsJSON = json.RawMessage(questions.String)
	}
	if len(answers) > 0 {
		r.UserAnswers = []string(answers)
	}
	if personality.Valid {
		r.PersonalityJSON = json.RawMessage(personality.String)
	}
	if totems.Valid {
		r.TotemsJSON = json.RawMessage(totems.String)
	}
	if ranking.Valid {
		r.RankingJSON = json.RawMessage(ranking.String)
	}
	if selectedTotem.Valid {
		v := selectedTotem.String
		r.SelectedTotemName = &v
	}
	if errMsg.Valid {
		v := errMsg.String
		r.ErrorMessage = &v
	}
	if phaseStartedAt.Valid {
		t := phaseStartedAt.Time
		r.PhaseStartedAt = &t
	}
	if completedAt.Valid {
		t := completedAt.Time
		r.CompletedAt = &t
	}
	return &r, nil
}

func (s *Store) CountAvatarReadingsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM avatar_readings
		WHERE user_id = $1
		  AND created_at >= $2
	`, userID, since).Scan(&count)
	return count, err
}

func (s *Store) GetAvatarReadingByID(ctx context.Context, id uuid.UUID) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+avatarReadingColumns+`
		FROM avatar_readings
		WHERE id = $1
	`, id)
	return scanAvatarReading(row)
}

func (s *Store) GetActiveAvatarReadingForUser(ctx context.Context, userID uuid.UUID) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+avatarReadingColumns+`
		FROM avatar_readings
		WHERE user_id = $1
		  AND completed_at IS NULL
		  AND status != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, ReadingStatusCompleted)
	return scanAvatarReading(row)
}

func (s *Store) getIncompleteAvatarReadingForUser(ctx context.Context, userID uuid.UUID) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+avatarReadingColumns+`
		FROM avatar_readings
		WHERE user_id = $1
		  AND completed_at IS NULL
		  AND status != $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, ReadingStatusCompleted)
	return scanAvatarReading(row)
}

func (s *Store) CreateAvatarReading(ctx context.Context, userID uuid.UUID, draw []int) (*AvatarReading, error) {
	if existing, err := s.getIncompleteAvatarReadingForUser(ctx, userID); err == nil && existing != nil {
		if existing.Status != ReadingStatusFailed {
			return nil, ErrReadingInProgress
		}
		if err := s.ArchiveAvatarReading(ctx, existing.ID); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, ErrReadingNotFound) {
		return nil, err
	}

	draw32 := make([]int32, len(draw))
	for i, n := range draw {
		draw32[i] = int32(n)
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO avatar_readings (user_id, status, draw, phase_started_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING `+avatarReadingColumns+`
	`, userID, ReadingStatusGeneratingQuestions, pq.Array(draw32))
	return scanAvatarReading(row)
}

func (s *Store) UpdateAvatarReadingStatus(ctx context.Context, id uuid.UUID, status string, errMsg *string) (*AvatarReading, error) {
	phaseStartSQL := ""
	if status == ReadingStatusGeneratingQuestions || status == ReadingStatusProcessing {
		phaseStartSQL = ", phase_started_at = NOW()"
	}
	row := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		UPDATE avatar_readings
		SET status = $2,
		    error_message = $3,
		    updated_at = NOW()%s
		WHERE id = $1 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, phaseStartSQL), id, status, errMsg)
	return scanAvatarReading(row)
}

// ArchiveAvatarReading hides an incomplete reading so the user can start fresh.
func (s *Store) ArchiveAvatarReading(ctx context.Context, id uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE avatar_readings
		SET completed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result, ErrReadingNotFound)
}

func (s *Store) SaveAvatarReadingQuestions(ctx context.Context, id uuid.UUID, questions json.RawMessage) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE avatar_readings
		SET questions_json = $2,
		    status = $3,
		    error_message = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, id, questions, ReadingStatusAwaitingAnswers)
	return scanAvatarReading(row)
}

func (s *Store) SaveAvatarReadingAnswers(ctx context.Context, id uuid.UUID, answers []string) (*AvatarReading, error) {
	if len(answers) != 5 {
		return nil, ErrInvalidReadingAnswer
	}
	for _, answer := range answers {
		switch strings.ToUpper(strings.TrimSpace(answer)) {
		case "A", "B", "C", "D", "E":
		default:
			return nil, ErrInvalidReadingAnswer
		}
	}

	row := s.db.QueryRowContext(ctx, `
		UPDATE avatar_readings
		SET user_answers = $2,
		    status = $3,
		    error_message = NULL,
		    phase_started_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = $4 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, id, pq.Array(answers), ReadingStatusProcessing, ReadingStatusAwaitingAnswers)
	reading, err := scanAvatarReading(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReadingNotReady
	}
	return reading, err
}

func (s *Store) SaveAvatarReadingPersonality(ctx context.Context, id uuid.UUID, personality json.RawMessage) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE avatar_readings
		SET personality_json = $2,
		    updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, id, personality)
	return scanAvatarReading(row)
}

func (s *Store) SaveAvatarReadingTotems(ctx context.Context, id uuid.UUID, totems json.RawMessage) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE avatar_readings
		SET totems_json = $2,
		    updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, id, totems)
	return scanAvatarReading(row)
}

func (s *Store) SaveAvatarReadingRanking(ctx context.Context, id uuid.UUID, ranking json.RawMessage) (*AvatarReading, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE avatar_readings
		SET ranking_json = $2,
		    status = $3,
		    error_message = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND completed_at IS NULL
		RETURNING `+avatarReadingColumns+`
	`, id, ranking, ReadingStatusReady)
	return scanAvatarReading(row)
}

func (s *Store) FailAvatarReading(ctx context.Context, id uuid.UUID, message string) (*AvatarReading, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "Spirit animal reading failed"
	}
	return s.UpdateAvatarReadingStatus(ctx, id, ReadingStatusFailed, &msg)
}

func (s *Store) InsertAvatarRender(ctx context.Context, readingID uuid.UUID, totemName string, artVersion int, imageURL, imagePrompt string) (*AvatarRender, error) {
	var render AvatarRender
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO avatar_renders (reading_id, totem_name, art_direction_version, image_url, image_prompt)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, reading_id, totem_name, art_direction_version, image_url, image_prompt, created_at
	`, readingID, totemName, artVersion, imageURL, imagePrompt).Scan(
		&render.ID,
		&render.ReadingID,
		&render.TotemName,
		&render.ArtDirectionVersion,
		&render.ImageURL,
		&render.ImagePrompt,
		&render.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &render, nil
}

func (s *Store) ListAvatarRendersForReading(ctx context.Context, readingID uuid.UUID) ([]AvatarRender, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, reading_id, totem_name, art_direction_version, image_url, image_prompt, created_at
		FROM avatar_renders
		WHERE reading_id = $1
		ORDER BY created_at ASC
	`, readingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AvatarRender
	for rows.Next() {
		var render AvatarRender
		if err := rows.Scan(
			&render.ID,
			&render.ReadingID,
			&render.TotemName,
			&render.ArtDirectionVersion,
			&render.ImageURL,
			&render.ImagePrompt,
			&render.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, render)
	}
	return out, rows.Err()
}

func (s *Store) ListIncompleteAvatarReadings(ctx context.Context) ([]*AvatarReading, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+avatarReadingColumns+`
		FROM avatar_readings
		WHERE completed_at IS NULL
		  AND status IN ($1, $2)
		ORDER BY updated_at ASC
	`, ReadingStatusGeneratingQuestions, ReadingStatusProcessing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*AvatarReading
	for rows.Next() {
		reading, err := scanAvatarReading(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, reading)
	}
	return out, rows.Err()
}

func (s *Store) GetLastCompletedSpiritAnimalAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var completedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT completed_at
		FROM avatar_readings
		WHERE user_id = $1
		  AND status = $2
		  AND completed_at IS NOT NULL
		ORDER BY completed_at DESC
		LIMIT 1
	`, userID, ReadingStatusCompleted).Scan(&completedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !completedAt.Valid {
		return nil, nil
	}
	t := completedAt.Time
	return &t, nil
}

func (s *Store) SelectSpiritAnimalTotem(ctx context.Context, userID, readingID uuid.UUID, totemName, imageURL string) (*User, error) {
	totemName = strings.TrimSpace(totemName)
	if totemName == "" {
		return nil, fmt.Errorf("store: totem name required")
	}
	url := strings.TrimSpace(imageURL)
	if url == "" {
		return nil, fmt.Errorf("store: image url required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var readingStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM avatar_readings
		WHERE id = $1 AND user_id = $2
	`, readingID, userID).Scan(&readingStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrReadingNotFound
		}
		return nil, err
	}
	if readingStatus != ReadingStatusReady {
		return nil, ErrReadingNotReady
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx, `
		UPDATE avatar_readings
		SET selected_totem_name = $2,
		    status = $3,
		    completed_at = $4,
		    updated_at = $4
		WHERE id = $1
	`, readingID, totemName, ReadingStatusCompleted, now)
	if err != nil {
		return nil, err
	}

	source := SourceSpiritAnimal
	row := tx.QueryRowContext(ctx, `
		UPDATE users
		SET avatar_url = $2,
		    avatar_key = NULL,
		    avatar_source = $3,
		    avatar_reading_id = $4,
		    updated_at = NOW()
		WHERE id = $1 AND is_active = true
		RETURNING `+userColumns+`
	`, userID, url, source, readingID)
	user, err := scanUser(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return user, nil
}
