package integration_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductRelations_OrderGuard(t *testing.T) {
	env := newTestEnv(t)
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")
	for _, kind := range []string{"exam", "course"} {
		for _, status := range []string{"none", "cart", "payment_expired", "cancelled", "payment_pending", "paid", "processing", "shipped", "completed"} {
			t.Run(kind+"/"+status, func(t *testing.T) {
				id, ids := relationProduct(t, env, kind)
				if status != "none" {
					student := seedUser(t, env, "student", "active", false)
					order := seedPaidOrder(t, env, student, id, kind)
					_, err := env.pool.Exec(context.Background(), `UPDATE orders SET status=$1 WHERE id=$2`, status, order)
					require.NoError(t, err)
				}
				resp, out := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/products/"+id+"/relations", map[string]any{kind + "_ids": ids[2:]}, token)
				if status == "none" || status == "cart" || status == "payment_expired" || status == "cancelled" {
					require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
					assert.ElementsMatch(t, ids[2:], persistedRelations(t, env, id, kind))
				} else {
					require.Equal(t, http.StatusConflict, resp.StatusCode, "body=%v", out)
					assert.Equal(t, "product_relations_locked", out["code"])
					assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, kind))
				}
			})
		}
	}
}

func TestProductRelations_AdminExamCanChange(t *testing.T) {
	env := newTestEnv(t)
	token := authToken(t, env, seedUser(t, env, "admin_exam", "active", false), "admin_exam")
	for _, kind := range []string{"exam", "course"} {
		t.Run(kind, func(t *testing.T) {
			id, ids := relationProduct(t, env, kind)
			resp, out := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/products/"+id+"/relations", map[string]any{kind + "_ids": ids[2:]}, token)
			require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
			assert.ElementsMatch(t, ids[2:], persistedRelations(t, env, id, kind))
		})
	}
}

func TestProductRelations_RolesAndValidation(t *testing.T) {
	env := newTestEnv(t)
	id, ids := relationProduct(t, env, "exam")
	for _, role := range []string{"admin_store", "admin_school", "student"} {
		t.Run(role, func(t *testing.T) {
			token := authToken(t, env, seedUser(t, env, role, "active", false), role)
			resp, out := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/products/"+id+"/relations", map[string]any{"exam_ids": ids[2:]}, token)
			require.Equal(t, http.StatusForbidden, resp.StatusCode, "body=%v", out)
			assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, "exam"))
		})
	}
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")
	courseID, courseIDs := relationProduct(t, env, "course")
	book := seedProduct(t, env, "book", "Book", 1000)
	for _, tt := range []struct {
		name, id string
		body     map[string]any
		status   int
	}{
		{"mismatch", id, map[string]any{"course_ids": courseIDs}, 400},
		{"course mismatch", courseID, map[string]any{"exam_ids": ids}, 400},
		{"wrong null key", id, map[string]any{"exam_ids": ids, "course_ids": nil}, 400},
		{"both keys", id, map[string]any{"exam_ids": ids, "course_ids": courseIDs}, 400},
		{"empty", id, map[string]any{"exam_ids": []string{}}, 400},
		{"course empty", courseID, map[string]any{"course_ids": []string{}}, 400},
		{"null", id, map[string]any{"exam_ids": nil}, 400},
		{"missing", id, map[string]any{}, 400},
		{"physical", book, map[string]any{"exam_ids": ids}, 400},
		{"unknown", uuid.NewString(), map[string]any{"exam_ids": ids}, 404},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, out := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/products/"+tt.id+"/relations", tt.body, token)
			require.Equal(t, tt.status, resp.StatusCode, "body=%v", out)
			assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, "exam"))
			assert.ElementsMatch(t, courseIDs[:2], persistedRelations(t, env, courseID, "course"))
		})
	}
}

func TestProductRelations_ConcurrentChangesAreWholeSets(t *testing.T) {
	env := newTestEnv(t)
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")
	for _, kind := range []string{"exam", "course"} {
		t.Run(kind, func(t *testing.T) {
			id, ids := relationProduct(t, env, kind)
			ctx := context.Background()
			tx, err := env.pool.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			var lockedID string
			require.NoError(t, tx.QueryRow(ctx, `SELECT id FROM product WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID))
			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, set := range [][]string{ids[:2], ids[2:]} {
				wg.Add(1)
				go func(set []string) {
					defer wg.Done()
					<-start
					resp, out := doJSONBody(t, env, http.MethodPut, "/api/v1/admin/products/"+id+"/relations", map[string]any{kind + "_ids": set}, token)
					assert.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
				}(set)
			}
			close(start)
			require.Eventually(t, func() bool {
				var waiting int
				err := env.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%SELECT type FROM product%'`).Scan(&waiting)
				return err == nil && waiting == 2
			}, 5*time.Second, 10*time.Millisecond, "both relation changes must wait for the product row lock")
			assert.ElementsMatch(t, ids[:2], persistedRelations(t, env, id, kind))
			require.NoError(t, tx.Commit(ctx))
			wg.Wait()
			got := persistedRelations(t, env, id, kind)
			if len(got) == 2 {
				assert.ElementsMatch(t, ids[:2], got)
			} else {
				assert.ElementsMatch(t, ids[2:], got)
			}
		})
	}
}

func TestProductRelations_DetailCarriesTitlesForAdminsOnly(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	for _, kind := range []string{"exam", "course"} {
		t.Run(kind, func(t *testing.T) {
			titles := map[string]string{}
			var ids []string
			for _, title := range []string{"Alpha " + kind, "Beta " + kind} {
				var id string
				if kind == "exam" {
					exam, err := env.svc.CreateExam(ctx, model.Exam{Title: title, Mode: "standard"})
					require.NoError(t, err)
					id = exam.ID.String()
				} else {
					course, err := env.svc.CreateCourse(ctx, title, "SMA", "Math", "Instructor", service.RoleSuperAdmin)
					require.NoError(t, err)
					id = course.ID.String()
				}
				titles[id] = title
				ids = append(ids, id)
			}
			p := model.Product{Type: kind, Name: "Titled", Price: 1000, Status: "published"}
			var product model.Product
			var err error
			if kind == "exam" {
				product, err = env.svc.CreateProductWithExams(ctx, p, ids, service.RoleSuperAdmin)
			} else {
				product, err = env.svc.CreateProductWithCourses(ctx, p, ids, service.RoleSuperAdmin)
			}
			require.NoError(t, err)

			admin, err := env.svc.GetProduct(ctx, product.ID, service.RoleSuperAdmin)
			require.NoError(t, err)
			got := map[string]string{}
			for _, r := range admin.Relations {
				got[r.ID] = r.Title
			}
			assert.Equal(t, titles, got)

			for _, role := range []string{service.RoleStudent, ""} {
				public, err := env.svc.GetProduct(ctx, product.ID, role)
				require.NoError(t, err)
				assert.Empty(t, public.Relations, "role=%q", role)
				if kind == "exam" {
					assert.ElementsMatch(t, ids, public.ExamIDs, "role=%q", role)
				} else {
					assert.ElementsMatch(t, ids, public.CourseIDs, "role=%q", role)
				}
			}
		})
	}
}
