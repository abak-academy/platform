package server

import (
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/service"
	"github.com/labstack/echo/v4"
)

func registerRoutes(e *echo.Echo, h *handler.Handler, svc *service.Service, jwtSigner *infra.JWTSigner) {
	v1 := e.Group("/api/v1")
	v1.GET("/health", h.Health)

	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login, handler.LoginRateLimiter())
	auth.POST("/google", h.GoogleLogin)
	auth.POST("/otp/send", h.SendOTP)
	auth.POST("/otp/verify", h.VerifyOTP)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout, handler.JWTMiddleware(svc, jwtSigner))
	auth.POST("/password/forgot", h.ForgotPassword)
	auth.POST("/password/reset", h.ResetPassword)
	auth.PATCH("/password/change", h.ChangePassword, handler.JWTMiddleware(svc, jwtSigner))
	auth.GET("/me", h.Me, handler.JWTMiddleware(svc, jwtSigner))
	auth.PATCH("/photo", h.UpdatePhoto, handler.JWTMiddleware(svc, jwtSigner))

	// Student product routes
	products := v1.Group("/products")
	products.GET("", h.ListProducts)
	products.GET("/:id", h.GetProduct)

	// Admin product routes
	admin := v1.Group("/admin")
	admin.Use(handler.JWTMiddleware(svc, jwtSigner))

	adminProducts := admin.Group("/products")
	adminProducts.GET("", h.AdminListProducts)
	adminProducts.POST("", h.AdminCreateProduct)
	adminProducts.GET("/:id", h.AdminGetProduct)
	adminProducts.PATCH("/:id", h.AdminUpdateProduct)
	adminProducts.POST("/:id/publish", h.AdminPublishProduct)
	adminProducts.DELETE("/:id", h.AdminDeleteProduct)

	// Admin course CRUD
	adminCourses := admin.Group("/courses")
	adminCourses.GET("", h.AdminListCourses)
	adminCourses.POST("", h.AdminCreateCourse)
	adminCourses.GET("/:id", h.AdminGetCourse)
	adminCourses.PATCH("/:id", h.AdminUpdateCourse)
	adminCourses.DELETE("/:id", h.AdminDeleteCourse)

	// Admin section routes (re-keyed from product to course)
	adminSections := admin.Group("/courses/:id/sections")
	adminSections.GET("", h.AdminListSections)
	adminSections.POST("", h.AdminCreateSection)
	adminSections.PUT("/:sId", h.AdminUpdateSection)
	adminSections.DELETE("/:sId", h.AdminDeleteSection)
	adminSections.PATCH("/reorder", h.AdminReorderSections)

	// Admin lesson routes (re-keyed from product to course)
	adminLessons := admin.Group("/courses/:id/sections/:sId/lessons")
	adminLessons.POST("", h.AdminCreateLesson)
	adminLessons.PUT("/:lId", h.AdminUpdateLesson)
	adminLessons.DELETE("/:lId", h.AdminDeleteLesson)
	adminLessons.PATCH("/reorder", h.AdminReorderLessons)

	// Student order routes
	orders := v1.Group("/orders")
	orders.Use(handler.JWTMiddleware(svc, jwtSigner))
	orders.POST("", h.MintCart)
	orders.GET("", h.GetOrders)
	orders.GET("/:id", h.GetOrder)
	orders.POST("/:id/items", h.AddItem)
	orders.DELETE("/:id/items/:itemId", h.RemoveItem)
	orders.PATCH("/:id/items/:itemId", h.UpdateItemQty)
	orders.PATCH("/:id", h.PatchCart)
	orders.POST("/:id/checkout", h.Checkout)
	orders.POST("/:id/retry", h.RetryPayment)

	// Shipping and promo routes
	shipping := v1.Group("/orders/shipping")
	shipping.POST("", h.GetShipping)

	promo := v1.Group("/promo-codes")
	promo.POST("/validate", h.ValidatePromo)

	// Active public promo listing — sibling group, same prefix, so JWT auth
	// here does not leak onto the anonymous /promo-codes/validate route above.
	promoAuth := v1.Group("/promo-codes")
	promoAuth.Use(handler.JWTMiddleware(svc, jwtSigner))
	promoAuth.GET("/active", h.ListActivePublicPromos)

	// Payment webhook route (no auth, uses HMAC signature)
	webhooks := v1.Group("/webhooks")
	webhooks.POST("/payment", h.HandlePaymentWebhook)
	// Shipping webhook route (no auth by route; gated by HandleShippingWebhook's
	// constant-time X-Biteship-Signature check, which fails closed).
	webhooks.POST("/shipping", h.HandleShippingWebhook)

	// Public config (client key is safe to expose)
	v1.GET("/config/payment-client-key", h.GetPaymentClientKey)

	// Public school list
	v1.GET("/schools", h.ListSchools)

	// Public region reference data (no auth, mirrors GET /schools)
	v1.GET("/provinces", h.ListProvinces)
	v1.GET("/provinces/:id/cities", h.ListCitiesByProvince)
	v1.GET("/cities/:id/districts", h.ListDistrictsByCity)

	// Avatar read-proxy (no auth; service restricts to the avatars/ prefix)
	v1.GET("/files/*", h.ServeFile)

	// Upload presign (authenticated)
	uploads := v1.Group("/uploads")
	uploads.Use(handler.JWTMiddleware(svc, jwtSigner))
	uploads.POST("/presign", h.GeneratePresignUploadURL)

	// Student profile routes
	students := v1.Group("/students")
	students.Use(handler.JWTMiddleware(svc, jwtSigner))
	students.Use(handler.StudentOnlyMiddleware())
	students.GET("/dashboard", h.StudentDashboard)
	students.GET("/profile", h.StudentProfile)
	students.PATCH("/profile", h.StudentUpdateProfile)
	students.PATCH("/photo", h.UpdatePhoto)

	// Student course routes
	studentCourses := v1.Group("/courses")
	studentCourses.Use(handler.JWTMiddleware(svc, jwtSigner))
	studentCourses.GET("", h.StudentListCourses)
	studentCourses.GET("/:id", h.StudentGetCourse)
	studentCourses.POST("/:id/lessons/:lId/complete", h.StudentMarkLessonComplete)
	studentCourses.GET("/:id/progress", h.StudentCourseProgress)

	// Student exam routes (registrations + card download)
	exam := v1.Group("/exam")
	exam.Use(handler.JWTMiddleware(svc, jwtSigner))
	exam.GET("/registrations", h.StudentListRegistrations)
	exam.GET("/registrations/:id", h.StudentGetRegistration)
	exam.GET("/registrations/:id/card", h.StudentGetExamCard)
	exam.POST("/checkin", h.StudentCheckIn)
	exam.POST("/sessions", h.StudentStartSession)
	exam.GET("/sessions/:id", h.StudentReconnectSession)
	exam.PATCH("/sessions/:id/answers", h.StudentSaveAnswers)
	exam.POST("/sessions/:id/submit", h.StudentSubmitSession)
	exam.POST("/sessions/:id/sections/:testId/advance", h.StudentAdvanceSection)
	exam.POST("/sessions/:id/violations", h.StudentLogViolation)
	exam.GET("/sessions/:id/result", h.StudentGetSessionResult)
	exam.GET("/sessions/:id/leaderboard", h.StudentGetSessionLeaderboard)

	// Admin session routes (exam proctoring)
	adminSessions := admin.Group("/sessions")
	adminSessions.Use(handler.RBACMiddleware("sessions:write"))
	adminSessions.POST("/:id/reopen", h.AdminReopenSession)
	adminSessions.POST("/:id/force-submit", h.AdminForceSubmitSession)
	adminSessions.GET("/:id/essays", h.AdminGetSessionEssays)
	adminSessions.POST("/:id/grade", h.AdminGradeEssay)

	// Admin session routes — read-only group (sibling, same path prefix)
	adminSessionsRead := admin.Group("/sessions")
	adminSessionsRead.Use(handler.RBACMiddleware("sessions:read"))
	adminSessionsRead.GET("/monitor", h.AdminGetSessionMonitor)
	adminSessionsRead.GET("/monitor/available", h.AdminListExamsForMonitor)
	adminSessionsRead.GET("/:id/violations", h.AdminGetSessionViolations)

	// Admin order routes
	adminOrders := admin.Group("/orders")
	adminOrders.Use(handler.RBACMiddleware("orders:write"))
	adminOrders.GET("", h.AdminListOrders)
	// Registered before /:id so "summary" is never read as an order id.
	adminOrders.GET("/summary", h.AdminOrdersSummary)
	adminOrders.GET("/:id", h.AdminGetOrder)
	adminOrders.GET("/:id/payment-proof", h.AdminGetPaymentProof)
	adminOrders.GET("/:id/refund-proof", h.AdminGetRefundProof)
	adminOrders.POST("/:id/confirm", h.AdminConfirmOrder)
	adminOrders.POST("/:id/ship", h.AdminShipOrder)
	adminOrders.POST("/:id/ship-manual", h.AdminShipOrderManual)
	adminOrders.GET("/:id/tracking", h.AdminGetOrderTracking)
	adminOrders.POST("/:id/shipment/refresh", h.AdminRefreshShipment)
	adminOrders.POST("/:id/shipment/cancel", h.AdminCancelShipment)
	adminOrders.GET("/:id/label", h.AdminGetShippingLabel)
	adminOrders.POST("/:id/complete", h.AdminCompleteOrder)
	adminOrders.POST("/:id/refund", h.AdminRefundOrder)
	adminOrders.POST("/:id/reconcile", h.AdminReconcileOrder)

	// Admin promo code routes
	adminPromos := admin.Group("/promo-codes")
	adminPromos.Use(handler.RBACMiddleware("promos:write"))
	adminPromos.GET("", h.AdminListPromoCodes)
	adminPromos.POST("", h.AdminCreatePromoCode)
	adminPromos.PUT("/:id", h.AdminUpdatePromoCode)
	adminPromos.DELETE("/:id", h.AdminDeletePromoCode)

	// Admin revenue and notification routes
	adminRevenue := admin.Group("/revenue")
	adminRevenue.Use(handler.RBACMiddleware("revenue:read"))
	adminRevenue.GET("", h.AdminGetRevenue)

	adminNotifs := admin.Group("/notifications")
	adminNotifs.Use(handler.RBACMiddleware("notifications:read"))
	adminNotifs.GET("", h.AdminListNotifications)
	adminNotifs.PATCH("/:id/read", h.AdminMarkNotificationRead)
	// Announcement CRUD + send
	adminNotifs.POST("/announcements", h.AdminCreateAnnouncement, handler.RBACMiddleware("notifications:write"))
	adminNotifs.GET("/announcements", h.AdminListAnnouncements, handler.RBACMiddleware("notifications:read"))
	adminNotifs.PATCH("/announcements/:id", h.AdminUpdateAnnouncement, handler.RBACMiddleware("notifications:write"))
	adminNotifs.DELETE("/announcements/:id", h.AdminDeleteAnnouncement, handler.RBACMiddleware("notifications:write"))
	adminNotifs.POST("/announcements/:id/send", h.AdminSendAnnouncement, handler.RBACMiddleware("notifications:write"))

	// Admin school routes
	adminSchools := admin.Group("/schools")
	adminSchools.Use(handler.RBACMiddleware("schools:write"))
	adminSchools.GET("", h.AdminListSchools)
	adminSchools.GET("/options", h.AdminListSchoolOptions)
	adminSchools.POST("", h.AdminCreateSchool)
	adminSchools.PUT("/:id", h.AdminUpdateSchool)
	adminSchools.PATCH("/:id", h.AdminChangeSchoolStatus)
	adminSchools.POST("/bulk/presign", h.AdminPresignSchoolBulkUpload)
	adminSchools.POST("/bulk", h.AdminBulkImportSchools)

	// Admin student routes (row-scoped via JWT schoolID)
	adminStudents := admin.Group("/students")
	adminStudents.Use(handler.RBACMiddleware("students:*"))
	adminStudents.GET("", h.AdminListStudents)
	adminStudents.POST("", h.AdminRegisterStudent)
	adminStudents.PATCH("/:id", h.AdminChangeStudentStatus)
	adminStudents.GET("/:id/credentials", h.AdminGetStudentCredentials)
	adminStudents.POST("/bulk/presign", h.AdminPresignStudentBulkUpload)
	adminStudents.POST("/bulk", h.AdminBulkImportStudents)
	adminStudents.POST("/bulk/credentials", h.AdminBulkReissueCredentials)

	// Admin results routes (school-scoped exam results)
	adminResults := admin.Group("/results")
	adminResults.Use(handler.RBACMiddleware("results:read"))
	adminResults.GET("", h.AdminListResults)
	adminResults.GET("/export", h.AdminExportResults)
	adminResults.GET("/:session_id", h.AdminGetResultDetail)

	// Admin job routes (JWT-only; any authenticated user may poll their own job)
	adminJobs := admin.Group("/jobs")
	adminJobs.GET("/:id", h.AdminGetJob)

	// Admin exam routes (Tests + Questions)
	adminTests := admin.Group("/tests")
	adminTests.Use(handler.RBACMiddleware("tests:*"))
	adminTests.GET("", h.AdminListTests)
	adminTests.POST("", h.AdminCreateTest)
	adminTests.GET("/:id", h.AdminGetTest)
	adminTests.PATCH("/:id", h.AdminUpdateTest)
	adminTests.DELETE("/:id", h.AdminDeleteTest)
	adminTests.GET("/:id/questions", h.AdminListQuestions)
	adminTests.POST("/:id/questions", h.AdminCreateQuestion)
	adminTests.POST("/:id/questions/attach", h.AdminAttachQuestions)
	adminTests.DELETE("/:id/questions/:questionId", h.AdminDetachQuestion)
	adminTests.PUT("/:id/questions/order", h.AdminReorderTestQuestions)

	adminQuestions := admin.Group("/questions")
	adminQuestions.Use(handler.RBACMiddleware("questions:*"))
	adminQuestions.GET("", h.AdminListBankQuestions)
	adminQuestions.GET("/import-template", h.AdminGetQuestionImportTemplate)
	adminQuestions.POST("", h.AdminCreateBankQuestion)
	adminQuestions.POST("/import", h.AdminImportQuestions)
	adminQuestions.PATCH("/:id", h.AdminUpdateQuestion)
	adminQuestions.DELETE("/:id", h.AdminDeleteQuestion)

	// Admin topic routes (curated subject/name pairs for the question bank)
	adminTopics := admin.Group("/topics")
	adminTopics.Use(handler.RBACMiddleware("questions:*"))
	adminTopics.GET("", h.AdminListTopics)
	adminTopics.POST("", h.AdminCreateTopic)
	adminTopics.PATCH("/:id", h.AdminUpdateTopic)
	adminTopics.DELETE("/:id", h.AdminDeleteTopic)

	// Admin exam package routes. Selling an exam (price/status/publish) goes through
	// the generic /admin/products flow (exam_ids attach), not a route here — mirrors
	// how course-type products work.
	adminExams := admin.Group("/exams")
	adminExams.Use(handler.RBACMiddleware("products(exam):write"))
	adminExams.POST("", h.AdminCreateExam)
	adminExams.PATCH("/:id", h.AdminUpdateExam)
	adminExams.PUT("/:id/tests", h.AdminReplaceExamTests)
	adminExams.GET("/:id/grading", h.AdminListGradingSessions)
	adminExams.GET("/:id/leaderboard", h.AdminGetExamLeaderboard)
	adminExams.GET("/:id/analytics", h.AdminGetExamAnalytics)
	adminExams.POST("/:id/certificate-preview", h.AdminGetExamCertificatePreview)
	adminExams.GET("/:id/certificate-design", h.AdminGetExamCertificateDesign)
	adminExams.PUT("/:id/certificate-design", h.AdminUpdateExamCertificateDesign)
	adminExams.PATCH("/:id/certificate-enabled", h.AdminSetExamCertificateEnabled)
	adminExams.PATCH("/:id/card-enabled", h.AdminSetExamCardEnabled)
	adminExams.POST("/:id/certificate-assets/presign", h.AdminPresignExamCertificateAsset)

	// Admin exam routes — read-only group (sibling, same path prefix). admin_school
	// needs list/detail to use the Registrations tab on the exam detail page, but
	// not the content-management sub-resources above (those stay write-gated).
	adminExamsRead := admin.Group("/exams")
	adminExamsRead.Use(handler.RBACMiddleware("products(exam):read"))
	adminExamsRead.GET("", h.AdminListExams)
	adminExamsRead.GET("/:id", h.AdminGetExam)
	adminExamsRead.GET("/:id/registrations", h.AdminListExamRegistrations)
	adminExamsRead.GET("/:id/registrations/export", h.AdminExportExamRegistrations)

	// Admin exam results workspace (Issue 124) — super_admin only. "results-workspace:read"
	// is claimed by no other role, so RBACMiddleware rejects everyone else with 403
	// (mirrors the adminSystem group's "system:admin" idiom).
	adminResultsWorkspace := admin.Group("/exams")
	adminResultsWorkspace.Use(handler.RBACMiddleware("results-workspace:read"))
	adminResultsWorkspace.GET("/:id/results-workspace", h.AdminGetExamResultsWorkspace)
	adminResultsWorkspace.GET("/:id/results-workspace/:registration_id/attempts", h.AdminGetResultsWorkspaceAttempts)
	adminResultsWorkspace.GET("/:id/results-workspace/sessions/:session_id", h.AdminGetResultsWorkspaceResultDetail)

	// Admin upload routes (image + audio presigning)
	adminUploads := admin.Group("/uploads")
	adminUploads.Use(handler.RBACMiddleware("uploads:write"))
	adminUploads.POST("/image", h.AdminUploadImage)
	adminUploads.POST("/audio", h.AdminUploadAudio)

	// Admin exam grant routes (super_admin only — satisfies "exam-grants:write"
	// via the "*" wildcard; see rbac.go:29-48)
	adminExamGrants := admin.Group("/exam-grants")
	adminExamGrants.Use(handler.RBACMiddleware("exam-grants:write"))
	adminExamGrants.POST("", h.AdminGrantExamAccess)
	adminExamGrants.GET("/students/search", h.AdminSearchGrantStudents)
	adminExamGrants.POST("/bulk/presign", h.AdminPresignExamGrantBulkUpload)
	adminExamGrants.POST("/bulk", h.AdminEnqueueExamGrantBulk)

	// Admin bulk-exam-order routes (FR-BULK-01..07). admin_school +
	// super_admin + admin_store may all order exams; capability is
	// "bulk-exam-orders:write".
	adminBulkExamOrders := admin.Group("/bulk-exam-orders")
	adminBulkExamOrders.Use(handler.RBACMiddleware("bulk-exam-orders:write"))
	adminBulkExamOrders.GET("/exams", h.AdminListOrderableExams)
	adminBulkExamOrders.POST("/preview", h.AdminPreviewBulkOrder)
	adminBulkExamOrders.POST("", h.AdminCreateBulkOrder)
	adminBulkExamOrders.POST("/:id/checkout", h.AdminCheckoutBulkOrder)

	// Admin system routes (super_admin only)
	adminSystem := admin.Group("/system")
	adminSystem.Use(handler.RBACMiddleware("system:admin"))
	adminSystem.GET("/accounts", h.AdminListAccounts)
	adminSystem.POST("/accounts", h.AdminCreateAccount)
	adminSystem.PATCH("/accounts/:id/role", h.AdminChangeAccountRole)
	adminSystem.PATCH("/accounts/:id/status", h.AdminChangeAccountStatus)
	adminSystem.POST("/accounts/:id/reset-password", h.AdminResetAccountPassword)
	adminSystem.GET("/audit", h.AdminListAuditLog)
	adminSystem.GET("/config", h.AdminGetSystemConfig)
	adminSystem.PUT("/config", h.AdminUpdateSystemConfig)

	// Super-admin dashboard — one request for the whole page. revenue:read is
	// the same capability the page checks client-side, which keeps admin_store
	// out per the 2026-08-04 revenue split.
	adminDashboard := admin.Group("/dashboard")
	adminDashboard.Use(handler.RBACMiddleware("revenue:read"))
	adminDashboard.GET("", h.AdminDashboard)

	// Sibling group, not a sub-route of adminDashboard — a sub-route would
	// inherit revenue:read and lock out admin_exam, the role this is for.
	adminExamDashboard := admin.Group("/dashboard")
	adminExamDashboard.Use(handler.RBACMiddleware("sessions:read"))
	adminExamDashboard.GET("/exam", h.AdminExamDashboard)

	adminSchoolDashboard := admin.Group("/dashboard")
	adminSchoolDashboard.Use(handler.RBACMiddleware("students:read"))
	adminSchoolDashboard.GET("/school", h.AdminSchoolDashboard)
}

// RegisterRoutesForTest is the same as registerRoutes but exported for handler tests.
func RegisterRoutesForTest(e *echo.Echo, h *handler.Handler, svc *service.Service, jwtSigner *infra.JWTSigner) {
	registerRoutes(e, h, svc, jwtSigner)
}
