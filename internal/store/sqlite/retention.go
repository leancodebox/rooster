package sqlite

import (
	"context"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	storepkg "github.com/leancodebox/rooster/internal/store"
)

func (s *Store) RetentionCandidates(ctx context.Context, cutoff time.Time, keep int) ([]domain.Execution, error) {
	rows, err := s.db.QueryContext(ctx, executionSelect+` WHERE id IN (
SELECT id FROM (
 SELECT id,finished_at,status,ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY created_at DESC,id DESC) AS rank
 FROM executions
) WHERE rank > ? AND finished_at < ? AND status IN ('succeeded','failed','canceled')
) ORDER BY created_at ASC,id ASC`, keep, formatTime(cutoff))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Execution, 0)
	for rows.Next() {
		execution, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, execution)
	}
	return result, rows.Err()
}

func (s *Store) DeleteFinishedExecution(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM executions WHERE id=? AND status IN ('succeeded','failed','canceled')", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return storepkg.ErrNotFound
	}
	return nil
}
