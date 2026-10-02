package integration_test

import (
	"context"
	"net/http"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the REAL service + REAL Postgres (no shim/fake), so a
// regression in the production preservation logic would fail them. They
// replace the tautological shim/fake tests in internal/service/{store,course}_test.go.

// Finding 1 / item-13: editing only level must not blank the title (or other fields).
func TestUpdateCourse_PreservesTitleAndFields_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	course, err := env.svc.CreateCourse(ctx, "Matematika Dasar", "SMA", "Matematika", "Budi", service.RoleAdminStore)
	require.NoError(t, err)

	// Partial update: only level changes; title/subject/instructor sent as "".
	updated, err := env.svc.UpdateCourse(ctx, course.ID.String(), "", "SMP", "", "", service.RoleAdminStore)
	require.NoError(t, err)

	assert.Equal(t, "SMP", updated.Level, "level should update to SMP")
	assert.Equal(t, "Matematika Dasar", updated.Title, "title must be preserved, not blanked")
	assert.Equal(t, "Matematika", updated.Subject, "subject must be preserved")
	assert.Equal(t, "Budi", updated.InstructorName, "instructor must be preserved")

	// Verify the persisted row, not just the returned struct.
	var title, subject, instructor string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT title, subject, instructor_name FROM course WHERE id = $1`,
		course.ID).Scan(&title, &subject, &instructor))
	assert.Equal(t, "Matematika Dasar", title, "persisted title must be preserved")
	assert.Equal(t, "Matematika", subject, "persisted subject must be preserved")
	assert.Equal(t, "Budi", instructor, "persisted instructor must be preserved")
}

// Finding 6 / Bug C: UpdateProduct must preserve Type/WeightGrams/ImageURL from
// the stored row when the request omits them.
func TestUpdateProduct_PreservesTypeWeightImage_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created, err := env.svc.CreateProduct(ctx, model.Product{
		Type:        "book",
		Name:        "Buku Original",
		WeightGrams: 500,
		ImageURL:    "http://example.com/cover.jpg",
		Price:       10000,
		Stock:       10,
		Status:      "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)

	// Edit only Name; Type/WeightGrams/ImageURL are zero in the request.
	updated, err := env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name:   "Buku Renamed",
		Price:  10000,
		Stock:  10,
		Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)

	assert.Equal(t, "book", updated.Type, "type must be preserved")
	assert.Equal(t, 500, updated.WeightGrams, "weight_grams must be preserved")
	assert.Equal(t, "http://example.com/cover.jpg", updated.ImageURL, "image_url must be preserved")

	var ptype string
	var weight int
	var image string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT type, weight_grams, image_url FROM product WHERE id = $1`,
		created.ID).Scan(&ptype, &weight, &image))
	assert.Equal(t, "book", ptype)
	assert.Equal(t, 500, weight)
	assert.Equal(t, "http://example.com/cover.jpg", image)
}

// FR9: an update carrying a NEW weight_grams + image_url must persist both
// (not re-write the stored values), then a later update that omits them must
// preserve the last-persisted values.
func TestUpdateProduct_PersistsNewWeightImage_ThenPreservesOnOmit_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created, err := env.svc.CreateProduct(ctx, model.Product{
		Type:        "merchandise",
		Name:        "Academy Tee",
		WeightGrams: 200,
		ImageURL:    "http://example.com/old.jpg",
		Price:       50000,
		Stock:       10,
		Status:      "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)

	// Update with NEW weight + image — both must persist.
	updated, err := env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name:        "Academy Tee",
		WeightGrams: 350,
		ImageURL:    "http://example.com/new.jpg",
		Price:       50000,
		Stock:       10,
		Status:      "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)
	assert.Equal(t, 350, updated.WeightGrams, "new weight_grams must persist")
	assert.Equal(t, "http://example.com/new.jpg", updated.ImageURL, "new image_url must persist")

	var weight int
	var image string
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT weight_grams, image_url FROM product WHERE id = $1`,
		created.ID).Scan(&weight, &image))
	assert.Equal(t, 350, weight, "persisted weight_grams must be the new value")
	assert.Equal(t, "http://example.com/new.jpg", image, "persisted image_url must be the new value")

	// Second update omits both (zero/empty) — the last-persisted values survive.
	_, err = env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name:   "Academy Tee v2",
		Price:  50000,
		Stock:  10,
		Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)

	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT weight_grams, image_url FROM product WHERE id = $1`,
		created.ID).Scan(&weight, &image))
	assert.Equal(t, 350, weight, "omitted weight_grams must preserve the last-persisted value")
	assert.Equal(t, "http://example.com/new.jpg", image, "omitted image_url must preserve the last-persisted value")
}

func TestUpdateProduct_Course_PersistsNewWeightImage_ThenPreservesOnOmit_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	course, err := env.svc.CreateCourse(ctx, "Course Update", "SMA", "Matematika", "Budi", service.RoleAdminStore)
	require.NoError(t, err)
	created, err := env.svc.CreateProductWithCourses(ctx, model.Product{
		Type: "course", Name: "Course Bundle", WeightGrams: 200, ImageURL: "http://example.com/old-course.jpg",
		Price: 50000, Stock: 10, Status: "published",
	}, []string{course.ID.String()}, service.RoleAdminStore)
	require.NoError(t, err)

	updated, err := env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Course Bundle", WeightGrams: 350, WeightGramsSet: true, ImageURL: "http://example.com/new-course.jpg", ImageURLSet: true,
		Price: 50000, Stock: 10, Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)
	assert.Equal(t, 350, updated.WeightGrams)
	assert.Equal(t, "http://example.com/new-course.jpg", updated.ImageURL)

	_, err = env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Course Bundle v2", Price: 50000, Stock: 10, Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)
	assertPersistedProductWeightImage(t, env, created.ID, 350, "http://example.com/new-course.jpg")
	assert.ElementsMatch(t, []string{course.ID.String()}, persistedRelations(t, env, created.ID, "course"))
}

func TestUpdateProduct_Exam_PersistsNewWeightImage_ThenPreservesOnOmit_RealService(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	exam, err := env.svc.CreateExam(ctx, model.Exam{Title: "Exam Update", Mode: "standard"})
	require.NoError(t, err)
	created, err := env.svc.CreateProductWithExams(ctx, model.Product{
		Type: "exam", Name: "Exam Bundle", WeightGrams: 200, ImageURL: "http://example.com/old-exam.jpg",
		Price: 50000, Stock: 10, Status: "published",
	}, []string{exam.ID.String()}, service.RoleAdminStore)
	require.NoError(t, err)

	updated, err := env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Exam Bundle", WeightGrams: 350, WeightGramsSet: true, ImageURL: "http://example.com/new-exam.jpg", ImageURLSet: true,
		Price: 50000, Stock: 10, Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)
	assert.Equal(t, 350, updated.WeightGrams)
	assert.Equal(t, "http://example.com/new-exam.jpg", updated.ImageURL)

	_, err = env.svc.UpdateProduct(ctx, created.ID, model.Product{
		Name: "Exam Bundle v2", Price: 50000, Stock: 10, Status: "published",
	}, service.RoleAdminStore)
	require.NoError(t, err)
	assertPersistedProductWeightImage(t, env, created.ID, 350, "http://example.com/new-exam.jpg")
	assert.ElementsMatch(t, []string{exam.ID.String()}, persistedRelations(t, env, created.ID, "exam"))
}

func assertPersistedProductWeightImage(t *testing.T, env *testEnv, productID string, wantWeight int, wantImage string) {
	t.Helper()
	var weight int
	var image string
	require.NoError(t, env.pool.QueryRow(context.Background(),
		`SELECT weight_grams, image_url FROM product WHERE id = $1`, productID,
	).Scan(&weight, &image))
	assert.Equal(t, wantWeight, weight)
	assert.Equal(t, wantImage, image)
}

// Finding 7 / Bug D: Repository.UpdateLesson must preserve position when only
// title/video/duration are supplied (the SQL SET clause omits position).
func TestUpdateLesson_PreservesPosition_RealRepo(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	course, err := env.svc.CreateCourse(ctx, "Course L", "beginner", "math", "Mr. A", service.RoleAdminStore)
	require.NoError(t, err)
	sec, err := env.svc.CreateSection(ctx, course.ID.String(), "Intro", service.RoleAdminStore)
	require.NoError(t, err)

	l1, err := env.svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "Welcome", "https://v/1", 300, service.RoleAdminStore)
	require.NoError(t, err)
	require.Equal(t, 0, l1.Position)

	l2, err := env.svc.CreateLesson(ctx, course.ID.String(), sec.ID.String(), "Basics", "https://v/2", 400, service.RoleAdminStore)
	require.NoError(t, err)
	require.Equal(t, 1, l2.Position)

	// Call the real repository boundary (the Bug D fix lives in the SQL SET clause).
	repo := repository.New(env.pool)
	updated, err := repo.UpdateLesson(ctx, l2.ID, model.Lesson{
		Title:           "Basics Updated",
		VideoURL:        "https://v/2",
		DurationSeconds: 400,
	})
	require.NoError(t, err)

	assert.Equal(t, "Basics Updated", updated.Title)
	assert.Equal(t, 1, updated.Position, "position must be preserved, not reset to 0")

	// Verify the persisted row.
	var pos int
	require.NoError(t, env.pool.QueryRow(ctx,
		`SELECT position FROM lesson WHERE id = $1`, l2.ID).Scan(&pos))
	assert.Equal(t, 1, pos, "persisted position must be preserved")
}

func TestProductPatch_RelationsRejectedWithoutWrites(t *testing.T) {
	env := newTestEnv(t)
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")
	for _, kind := range []string{"exam", "course"} {
		t.Run(kind, func(t *testing.T) {
			id, ids := relationProduct(t, env, kind)
			for _, body := range []map[string]any{
				{"name": "Changed", "exam_ids": ids[2:3]},
				{"name": "Changed", "exam_ids": []string{}},
				{"name": "Changed", "course_ids": ids[2:3]},
				{"name": "Changed", "course_ids": []string{}},
			} {
				resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/products/"+id, body, token)
				require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%v", out)
				assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, kind))
				var name string
				require.NoError(t, env.pool.QueryRow(context.Background(), `SELECT name FROM product WHERE id=$1`, id).Scan(&name))
				assert.Equal(t, "Original", name)
			}
		})
	}
}

func TestProductPatch_MetadataAndNullPreserveRelations(t *testing.T) {
	env := newTestEnv(t)
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")
	for _, kind := range []string{"exam", "course"} {
		t.Run(kind, func(t *testing.T) {
			id, ids := relationProduct(t, env, kind)
			for _, nulls := range []bool{false, true} {
				body := map[string]any{"name": "Updated", "price": 2000, "status": "hidden", "stock": 7, "image_url": "new.jpg", "specs": []map[string]string{{"key": "format", "label": "Format", "value": "Online"}}}
				if nulls {
					body["exam_ids"] = nil
					body["course_ids"] = nil
				}
				resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/products/"+id, body, token)
				require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
				assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, kind))
				product, err := env.svc.GetProduct(context.Background(), id, service.RoleSuperAdmin)
				require.NoError(t, err)
				assert.Equal(t, "Updated", product.Name)
				assert.Equal(t, int64(2000), product.Price)
				assert.Equal(t, "hidden", product.Status)
				assert.Equal(t, 7, product.Stock)
				assert.Equal(t, "new.jpg", product.ImageURL)
				assert.Equal(t, 100, product.WeightGrams)
				assert.Equal(t, []model.ProductSpec{{Key: "format", Label: "Format", Value: "Online"}}, product.Specs)
			}
		})
	}
}
