package repository

import (
	"context"

	"github.com/google/uuid"
)

func (r *Repository) GetQuestionTestMap(ctx context.Context, examID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT tq.question_id, tq.test_id
		FROM test_question tq
		JOIN exam_test et ON et.test_id = tq.test_id
		WHERE et.exam_id = $1
		ORDER BY et.sort_order ASC`,
		examID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questionTest := make(map[uuid.UUID]uuid.UUID)
	for rows.Next() {
		var questionID, testID uuid.UUID
		if err := rows.Scan(&questionID, &testID); err != nil {
			return nil, err
		}
		questionTest[questionID] = testID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return questionTest, nil
}
