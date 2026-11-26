package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func seedPre0075Grouping(t *testing.T, pool *pgxpool.Pool, name string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO product_grouping (name, status, created_at) VALUES ($1, 'published', $2) RETURNING id`,
		name, createdAt,
	).Scan(&id))
	return id
}

func seedPre0075Exam(t *testing.T, pool *pgxpool.Pool, title string, grouped *uuid.UUID, cardKey string) (examID, regID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO exam (title) VALUES ($1) RETURNING id`, title).Scan(&examID))
	if grouped != nil || title == "Ungrouped Exam" {
		var productID uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO product (type, name, price, status, grouping_id) VALUES ('exam', $1, 0, 'published', $2) RETURNING id`,
			title, grouped,
		).Scan(&productID))
		_, err := pool.Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, productID, examID)
		require.NoError(t, err)
	}
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, card_key, participant_number)
		 VALUES ($1, $2, $3, $4, 7) RETURNING id`,
		seedMigrationStudent(t, pool, "Siswa "+title), examID, "tok-"+uuid.NewString(), cardKey,
	).Scan(&regID))
	return examID, regID
}

func TestMigration0075_EventDataMigrationAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "0074_order_participant_per_item.up.sql")

	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	sains1 := seedPre0075Grouping(t, pool, "Olimpiade Sains", base)
	sains2 := seedPre0075Grouping(t, pool, "Olimpiade Sains", base.Add(time.Hour))
	kangaroo := seedPre0075Grouping(t, pool, "Kangaroo Math", base.Add(2*time.Hour))
	short := seedPre0075Grouping(t, pool, "A1", base.Add(3*time.Hour))
	symbols := seedPre0075Grouping(t, pool, "!!!", base.Add(4*time.Hour))
	evA := seedPre0075Grouping(t, pool, "Event", base.Add(5*time.Hour))
	evB := seedPre0075Grouping(t, pool, "Event 2", base.Add(6*time.Hour))
	two := seedPre0075Grouping(t, pool, "AB", base.Add(7*time.Hour))

	groupedExam, groupedReg := seedPre0075Exam(t, pool, "Grouped Exam", &kangaroo, "cards/grouped.pdf")
	_, ungroupedReg := seedPre0075Exam(t, pool, "Ungrouped Exam", nil, "cards/ungrouped.pdf")
	_, noProductReg := seedPre0075Exam(t, pool, "Productless Exam", nil, "cards/productless.pdf")
	var memberProduct uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT product_id FROM product_exam WHERE exam_id = $1`, groupedExam).Scan(&memberProduct))

	var numberBefore int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT participant_number FROM exam_registration WHERE id = $1`, groupedReg).Scan(&numberBefore))

	applyMigrationFile(t, pool, "0075_event.up.sql")

	slug := func(id uuid.UUID) string {
		var s string
		require.NoError(t, pool.QueryRow(ctx, `SELECT slug FROM product_grouping WHERE id = $1`, id).Scan(&s))
		return s
	}
	prefix := func(id uuid.UUID) *string {
		var p *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT participant_prefix FROM product_grouping WHERE id = $1`, id).Scan(&p))
		return p
	}

	require.Equal(t, "olimpiade-sains", slug(sains1))
	require.Equal(t, "olimpiade-sains-2", slug(sains2), "same-name groupings get -2 by age")
	require.Equal(t, "kangaroo-math", slug(kangaroo))
	require.Equal(t, "a1", slug(short))
	require.Equal(t, "event", slug(symbols), "a name with no letters or digits falls back to 'event'")

	// "Event" (rn 2 of base 'event') generates 'event-2' and "Event 2" literally slugifies to it.
	require.NotEqual(t, slug(evA), slug(evB))
	require.Contains(t, slug(evA), "event-2-")
	require.Contains(t, slug(evB), "event-2-")

	var distinct, total int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(DISTINCT slug), count(*) FROM product_grouping`).Scan(&distinct, &total))
	require.Equal(t, total, distinct)
	var bad int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_grouping WHERE slug !~ '^[a-z0-9]+(-[a-z0-9]+)*$'`).Scan(&bad))
	require.Zero(t, bad, "every slug is a non-empty kebab string")

	require.Nil(t, prefix(sains1), "two groupings deriving OLIM leave both NULL")
	require.Nil(t, prefix(sains2))
	require.Equal(t, "KANG", *prefix(kangaroo))
	require.Nil(t, prefix(short), "one letter is below the 2-letter minimum")
	require.Nil(t, prefix(symbols), "no letters at all")
	require.Nil(t, prefix(evA), "Event and Event 2 both derive EVEN")
	require.Nil(t, prefix(evB))
	require.Equal(t, "AB", *prefix(two), "a 2-letter name is long enough and used whole")

	var hidden, withContent int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE NOT is_visible), count(*) FILTER (WHERE content <> '{}'::jsonb) FROM product_grouping`,
	).Scan(&hidden, &withContent))
	require.Equal(t, total, hidden, "every existing grouping stays hidden")
	require.Zero(t, withContent)

	var memberGrouping uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT grouping_id FROM product WHERE id = $1`, memberProduct).Scan(&memberGrouping))
	require.Equal(t, kangaroo, memberGrouping, "membership survives")

	var numberAfter int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT participant_number FROM exam_registration WHERE id = $1`, groupedReg).Scan(&numberAfter))
	require.Equal(t, 7, numberBefore)
	require.Equal(t, numberBefore, numberAfter, "stored participant numbers do not change")

	cardKey := func(id uuid.UUID) *string {
		var k *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT card_key FROM exam_registration WHERE id = $1`, id).Scan(&k))
		return k
	}
	require.Nil(t, cardKey(groupedReg), "cached card of a grouped exam is cleared")
	require.Equal(t, "cards/ungrouped.pdf", *cardKey(ungroupedReg))
	require.Equal(t, "cards/productless.pdf", *cardKey(noProductReg))

	applyMigrationFile(t, pool, "0075_event.down.sql")
	requireColumnExists(t, pool, "product_grouping", "slug", false)
	requireColumnExists(t, pool, "product_grouping", "participant_prefix", false)
	requireColumnExists(t, pool, "product", "stages", false)
	var tables int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_name IN ('product_grouping_schedule','exam_external_round','exam_external_result')`).Scan(&tables))
	require.Zero(t, tables)
	var survivors int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM product_grouping`).Scan(&survivors))
	require.Equal(t, total, survivors, "down keeps the original grouping rows")

	applyMigrationFile(t, pool, "0075_event.up.sql")
	require.Equal(t, "olimpiade-sains-2", slug(sains2), "up again re-derives the same slugs")
	requireColumnExists(t, pool, "exam_external_result", "registration_id", true)
}
