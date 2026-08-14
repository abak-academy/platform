package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// biodataFieldLabels maps a biodataMissingFields key to the Indonesian label
// used in BiodataIncompleteError's message.
var biodataFieldLabels = map[string]string{
	"school": "sekolah",
	"grade":  "kelas",
	"dob":    "tanggal lahir",
}

// biodataMissingFields reports which of school/grade/dob a student is missing
// for exam registration, in the order the message lists them.
func biodataMissingFields(u *model.User) []string {
	if u == nil {
		return []string{"school", "grade", "dob"}
	}
	var missing []string
	hasSchool := (u.SchoolID != nil && *u.SchoolID != "") ||
		(u.UnlistedSchoolName != nil && *u.UnlistedSchoolName != "")
	if !hasSchool {
		missing = append(missing, "school")
	}
	if u.Grade == nil {
		missing = append(missing, "grade")
	}
	if u.DOB == nil {
		missing = append(missing, "dob")
	}
	return missing
}

// biodataComplete reports whether a student has the biodata required to register
// for an exam: a school (listed or unlisted), a class/grade, and a date of birth.
func biodataComplete(u *model.User) bool {
	return len(biodataMissingFields(u)) == 0
}

// BiodataIncompleteError names exactly which biodata fields are missing, so a
// student who e.g. already has a school on file isn't told to go fill one in.
// It satisfies errors.Is(err, ErrBiodataIncomplete) so existing callers that
// only check the sentinel keep working.
type BiodataIncompleteError struct {
	Missing []string // subset of "school", "grade", "dob", in that order
}

func (e *BiodataIncompleteError) Error() string {
	labels := make([]string, len(e.Missing))
	for i, f := range e.Missing {
		labels[i] = biodataFieldLabels[f]
	}
	return fmt.Sprintf("lengkapi biodata (%s) sebelum mendaftar ujian", strings.Join(labels, ", "))
}

func (e *BiodataIncompleteError) Is(target error) bool {
	return target == ErrBiodataIncomplete
}

type PromoValidation struct {
	PromoID  uuid.UUID
	Code     string
	Discount float64
	Total    float64
}

// adminExamProductTypes is the read-side mirror of checkTypeRBAC's admin_exam
// arm. Keep the two in step: a type writable but not readable strands a product
// the moment it is created.
var adminExamProductTypes = []string{"course", "exam"}

// narrowToAllowedTypes intersects a caller-requested type with the role's
// allowlist. A request naming a type outside the allowlist yields a non-nil
// empty slice — no rows — rather than being ignored, which would widen the
// query back to every type.
func narrowToAllowedTypes(allowed []string, requested string) []string {
	if requested == "" {
		return allowed
	}
	for _, t := range allowed {
		if t == requested {
			return []string{requested}
		}
	}
	return []string{}
}

// productListFilterForRole narrows a product list filter to what role may see.
// It is a free function so the policy is unit-testable without a repository —
// s.storeRepo is a concrete type and cannot be faked.
func productListFilterForRole(filter repository.ProductFilter, role string) repository.ProductFilter {
	switch role {
	case RoleSuperAdmin:
		// no filter restrictions
	case RoleAdminStore:
		// no filter restrictions — manages book, course, exam
	case RoleAdminExam:
		// Read boundary mirrors checkTypeRBAC's admin_exam arm: this role owns
		// the digital catalogue only, so physical inventory — including its
		// drafts — must not come back here. Status is deliberately left alone;
		// AdminCreateProduct writes "draft", and hiding drafts would make every
		// product this role creates invisible to it.
		filter.Types = narrowToAllowedTypes(adminExamProductTypes, filter.Type)
		filter.Type = ""
	default: // student or ""
		filter.VisibleOnly = true
		filter.Status = "published"
	}
	return filter
}

func (s *Service) ListProducts(ctx context.Context, filter repository.ProductFilter, role string) ([]model.Product, string, error) {
	return s.storeRepo.ListProducts(ctx, productListFilterForRole(filter, role))
}

func (s *Service) GetProduct(ctx context.Context, id string, role string) (model.Product, error) {
	p, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Product{}, ErrProductNotFound
		}
		return model.Product{}, err
	}
	if role == RoleStudent || role == "" {
		if p.Status != "published" {
			return model.Product{}, ErrProductNotFound
		}
		now := time.Now()
		if (p.AvailableFrom != nil && now.Before(*p.AvailableFrom)) ||
			(p.AvailableUntil != nil && now.After(*p.AvailableUntil)) {
			return model.Product{}, ErrProductNotFound
		}
	}
	if p.Type == "course" {
		pID, err := parseUUID(p.ID)
		if err == nil {
			courses, err := s.storeRepo.GetCoursesByProductID(ctx, pID)
			if err == nil {
				for _, c := range courses {
					p.CourseIDs = append(p.CourseIDs, c.ID.String())
				}
			}
		}
	}
	if p.Type == "exam" {
		pID, err := parseUUID(p.ID)
		if err == nil {
			exams, err := s.storeRepo.GetExamsByProductID(ctx, pID)
			if err == nil {
				for _, e := range exams {
					p.ExamIDs = append(p.ExamIDs, e.ID.String())
				}
			}
		}
	}
	return *p, nil
}

func (s *Service) CreateProduct(ctx context.Context, p model.Product, role string) (model.Product, error) {
	if err := checkTypeRBAC(role, p.Type); err != nil {
		return model.Product{}, err
	}
	if err := s.storeRepo.CreateProduct(ctx, &p); err != nil {
		return model.Product{}, err
	}
	return p, nil
}

func (s *Service) CreateProductWithCourses(ctx context.Context, p model.Product, courseIDs []string, role string) (model.Product, error) {
	if err := checkTypeRBAC(role, p.Type); err != nil {
		return model.Product{}, err
	}

	if p.Type == "course" && len(courseIDs) < 1 {
		return model.Product{}, ErrCourseLinkRequired
	}

	var ids []uuid.UUID
	for _, cid := range courseIDs {
		parsed, err := parseUUID(cid)
		if err != nil {
			return model.Product{}, err
		}
		ids = append(ids, parsed)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Product{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CreateProductWithCourses(ctx, tx, &p, ids); err != nil {
		return model.Product{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Product{}, err
	}

	return p, nil
}

func (s *Service) CreateProductWithExams(ctx context.Context, p model.Product, examIDs []string, role string) (model.Product, error) {
	if err := checkTypeRBAC(role, p.Type); err != nil {
		return model.Product{}, err
	}

	if p.Type == "exam" && len(examIDs) < 1 {
		return model.Product{}, ErrExamLinkRequired
	}

	var ids []uuid.UUID
	for _, eid := range examIDs {
		parsed, err := parseUUID(eid)
		if err != nil {
			return model.Product{}, err
		}
		ids = append(ids, parsed)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Product{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CreateProductWithExams(ctx, tx, &p, ids); err != nil {
		return model.Product{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Product{}, err
	}

	return p, nil
}

func (s *Service) UpdateProductWithExams(ctx context.Context, id string, p model.Product, examIDs []string, role string) (model.Product, error) {
	existing, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Product{}, ErrProductNotFound
		}
		return model.Product{}, err
	}
	if err := checkTypeRBAC(role, existing.Type); err != nil {
		return model.Product{}, err
	}
	p.Type = existing.Type
	if !p.WeightGramsSet && p.WeightGrams == 0 {
		p.WeightGrams = existing.WeightGrams
	}
	if !p.ImageURLSet && p.ImageURL == "" {
		p.ImageURL = existing.ImageURL
	}
	if !p.AvailableFromSet {
		p.AvailableFrom = existing.AvailableFrom
	}
	if !p.AvailableUntilSet {
		p.AvailableUntil = existing.AvailableUntil
	}

	var ids []uuid.UUID
	for _, eid := range examIDs {
		parsed, err := parseUUID(eid)
		if err != nil {
			return model.Product{}, err
		}
		ids = append(ids, parsed)
	}

	pID, err := parseUUID(id)
	if err != nil {
		return model.Product{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Product{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.UpdateProductTx(ctx, tx, id, &p); err != nil {
		return model.Product{}, err
	}
	if err := s.storeRepo.ReplaceProductExams(ctx, tx, pID, ids); err != nil {
		return model.Product{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Product{}, err
	}

	p.ID = id
	p.ExamIDs = examIDs
	return p, nil
}

func (s *Service) UpdateProductWithCourses(ctx context.Context, id string, p model.Product, courseIDs []string, role string) (model.Product, error) {
	existing, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Product{}, ErrProductNotFound
		}
		return model.Product{}, err
	}
	if err := checkTypeRBAC(role, existing.Type); err != nil {
		return model.Product{}, err
	}
	p.Type = existing.Type
	if !p.WeightGramsSet && p.WeightGrams == 0 {
		p.WeightGrams = existing.WeightGrams
	}
	if !p.ImageURLSet && p.ImageURL == "" {
		p.ImageURL = existing.ImageURL
	}
	if !p.AvailableFromSet {
		p.AvailableFrom = existing.AvailableFrom
	}
	if !p.AvailableUntilSet {
		p.AvailableUntil = existing.AvailableUntil
	}

	var ids []uuid.UUID
	for _, cid := range courseIDs {
		parsed, err := parseUUID(cid)
		if err != nil {
			return model.Product{}, err
		}
		ids = append(ids, parsed)
	}

	pID, err := parseUUID(id)
	if err != nil {
		return model.Product{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.Product{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.UpdateProductTx(ctx, tx, id, &p); err != nil {
		return model.Product{}, err
	}
	if err := s.storeRepo.ReplaceProductCourses(ctx, tx, pID, ids); err != nil {
		return model.Product{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Product{}, err
	}

	p.ID = id
	p.CourseIDs = courseIDs
	return p, nil
}

func (s *Service) UpdateProduct(ctx context.Context, id string, p model.Product, role string) (model.Product, error) {
	existing, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Product{}, ErrProductNotFound
		}
		return model.Product{}, err
	}
	if err := checkTypeRBAC(role, existing.Type); err != nil {
		return model.Product{}, err
	}
	p.Type = existing.Type
	if !p.WeightGramsSet && p.WeightGrams == 0 {
		p.WeightGrams = existing.WeightGrams
	}
	if !p.ImageURLSet && p.ImageURL == "" {
		p.ImageURL = existing.ImageURL
	}
	if !p.AvailableFromSet {
		p.AvailableFrom = existing.AvailableFrom
	}
	if !p.AvailableUntilSet {
		p.AvailableUntil = existing.AvailableUntil
	}
	if err := s.storeRepo.UpdateProduct(ctx, id, &p); err != nil {
		return model.Product{}, err
	}
	p.ID = id
	return p, nil
}

func (s *Service) PublishProduct(ctx context.Context, id string, role string) error {
	existing, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	if err := checkTypeRBAC(role, existing.Type); err != nil {
		return err
	}
	// FR-19: an exam-type product can bundle more than one exam (product_exam
	// M:N); every attached sectioned exam (mode utbk|ielts) must pass its
	// section gate before the product can publish.
	if existing.Type == "exam" {
		pID, err := parseUUID(id)
		if err != nil {
			return err
		}
		exams, err := s.storeRepo.GetExamsByProductID(ctx, pID)
		if err != nil {
			return err
		}
		for _, exam := range exams {
			if !isSectionedMode(exam.Mode) {
				continue
			}
			detail, err := s.storeRepo.GetExamDetail(ctx, exam.ID)
			if err != nil {
				return err
			}
			if err := validatePublishSections(exam, detail.Tests); err != nil {
				return err
			}
		}
	}
	return s.storeRepo.PublishProduct(ctx, id)
}

func (s *Service) DeleteProduct(ctx context.Context, id string, role string) error {
	existing, err := s.storeRepo.GetProductByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	if err := checkTypeRBAC(role, existing.Type); err != nil {
		return err
	}
	if isPhysicalType(existing.Type) {
		return s.storeRepo.DeleteProduct(ctx, id)
	}
	return s.storeRepo.ArchiveProduct(ctx, id)
}

func (s *Service) ValidatePromo(ctx context.Context, code string, subtotal float64, shippingCost float64) (PromoValidation, error) {
	promo, err := s.storeRepo.GetPromoByCode(ctx, code)
	if err != nil {
		return PromoValidation{}, err
	}
	if promo.Code == "" {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(time.Now()) {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.MaxUses != nil && promo.UsedCount >= *promo.MaxUses {
		return PromoValidation{}, ErrInvalidPromo
	}
	if promo.MinOrderAmount != nil && subtotal < *promo.MinOrderAmount {
		return PromoValidation{}, ErrPromoMinOrder
	}

	var discount float64
	if promo.DiscountPercent != nil {
		discount = subtotal * (*promo.DiscountPercent / 100)
		if promo.MaxDiscountAmount != nil && discount > *promo.MaxDiscountAmount {
			discount = *promo.MaxDiscountAmount
		}
	} else if promo.DiscountAmount != nil {
		discount = *promo.DiscountAmount
		if discount > subtotal {
			discount = subtotal
		}
	}

	return PromoValidation{PromoID: promo.ID, Code: code, Discount: discount, Total: subtotal - discount + shippingCost}, nil
}

func (s *Service) GetShippingRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	if err := ValidateKodePos(req.DestinationPostalCode); err != nil {
		return nil, err
	}

	rates, clientErr := s.logisticsClient().GetRates(ctx, req)

	var flatRate int64
	if cfg, cfgErr := s.GetSystemConfig(ctx); cfgErr == nil {
		if raw := cfg["shipping_fallback_flat_rate"]; raw != "" {
			if _, scanErr := fmt.Sscanf(raw, "%d", &flatRate); scanErr != nil {
				flatRate = 0
			}
		}
	}

	resolved, cause, err := resolveShippingRates(rates, clientErr, flatRate)
	if cause != "" {
		slog.Warn("shipping quote degraded to fallback", "cause", cause)
	}
	return resolved, err
}

func (s *Service) MintCart(ctx context.Context, studentID string) (model.Order, bool, error) {
	id, err := parseUUID(studentID)
	if err != nil {
		return model.Order{}, false, err
	}
	return s.storeRepo.MintCart(ctx, id)
}

func (s *Service) AddItem(ctx context.Context, studentID, orderID, productID string, qty int) error {
	oID, err := parseUUID(orderID)
	if err != nil {
		return err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return err
	}
	pID, err := parseUUID(productID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if order.StudentID != sID {
		return ErrOrderNotFound
	}
	if order.Status != "cart" {
		return ErrOrderNotEditable
	}

	product, err := s.storeRepo.GetProductByID(ctx, pID.String())
	if err != nil {
		return err
	}
	if product == nil {
		return ErrProductNotFound
	}
	if isPhysicalType(product.Type) && product.Stock == 0 {
		return ErrOutOfStock
	}
	if err := ValidateItemQty(product.Type, qty); err != nil {
		return err
	}
	if !isPhysicalType(product.Type) {
		for _, existing := range order.Items {
			if existing.ProductID == pID {
				return ErrDigitalQtyLimit
			}
		}
	}

	item := model.OrderItem{
		ProductID:   pID,
		ProductType: product.Type,
		Name:        product.Name,
		UnitPrice:   float64(product.Price),
		Qty:         qty,
		WeightGrams: product.WeightGrams,
	}
	clearShipping := isPhysicalType(product.Type)
	return s.storeRepo.AddItem(ctx, oID, item, clearShipping)
}

func (s *Service) RemoveItem(ctx context.Context, studentID, orderID, itemID string) error {
	oID, err := parseUUID(orderID)
	if err != nil {
		return err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return err
	}
	iID, err := parseUUID(itemID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if order.StudentID != sID {
		return ErrOrderNotFound
	}

	clearShipping := false
	for _, item := range order.Items {
		if item.ID == iID && isPhysicalType(item.ProductType) {
			clearShipping = true
			break
		}
	}

	return s.storeRepo.RemoveItem(ctx, oID, iID, clearShipping)
}

func (s *Service) UpdateItemQty(ctx context.Context, studentID, orderID, itemID string, qty int) error {
	oID, err := parseUUID(orderID)
	if err != nil {
		return err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return err
	}
	iID, err := parseUUID(itemID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return err
	}
	if order.ID.String() == "" || order.StudentID != sID {
		return ErrOrderNotFound
	}
	if order.Status != "cart" {
		return ErrOrderNotEditable
	}

	clearShipping := false
	for _, item := range order.Items {
		if item.ID == iID {
			if err := ValidateItemQty(item.ProductType, qty); err != nil {
				return err
			}
			if isPhysicalType(item.ProductType) {
				clearShipping = true
			}
			break
		}
	}

	return s.storeRepo.UpdateItemQty(ctx, oID, iID, qty, clearShipping)
}

type CartPatch struct {
	ShippingAddress json.RawMessage
	Courier         string
	Service         string
	ShippingCost    float64
	ProvinceID      *string
	CityID          *string
	DistrictID      *string
	KodePos         *string
	PromoCode       *string
}

// nilIfEmpty treats a pointer to an empty string the same as an absent field,
// so partial address patches never write "" into a non-null FK column.
func nilIfEmpty(p *string) *string {
	if p != nil && *p == "" {
		return nil
	}
	return p
}

// validateAddressHierarchy confirms provinceID/cityID/districtID form a valid,
// consistent province → city → district chain before it's persisted or used
// to price a shipment.
func (s *Service) validateAddressHierarchy(ctx context.Context, provinceID, cityID, districtID string) error {
	prov, err := s.storeRepo.GetProvinceByID(ctx, provinceID)
	if err != nil {
		return err
	}
	if prov == nil {
		return ErrInvalidProvinsi
	}

	city, err := s.storeRepo.GetCityByID(ctx, cityID)
	if err != nil {
		return err
	}
	if city == nil || city.ProvinceID != provinceID {
		return ErrInvalidKota
	}

	district, err := s.storeRepo.GetDistrictByID(ctx, districtID)
	if err != nil {
		return err
	}
	if district == nil || district.CityID != cityID {
		return ErrInvalidKecamatan
	}
	return nil
}

func (s *Service) PatchCart(ctx context.Context, studentID, orderID string, patch CartPatch) error {
	oID, err := parseUUID(orderID)
	if err != nil {
		return err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if order.StudentID != sID {
		return ErrOrderNotFound
	}
	if order.Status != "cart" {
		return ErrOrderNotEditable
	}

	patch.ProvinceID = nilIfEmpty(patch.ProvinceID)
	patch.CityID = nilIfEmpty(patch.CityID)
	patch.DistrictID = nilIfEmpty(patch.DistrictID)
	patch.KodePos = nilIfEmpty(patch.KodePos)

	if patch.KodePos != nil {
		if err := ValidateKodePos(*patch.KodePos); err != nil {
			return err
		}
	}

	// shipping_cost is never trusted from the client: it is either recomputed
	// server-side from a live courier quote (below) or carried over unchanged
	// from the persisted order, never taken from patch.ShippingCost directly.
	repoPatch := repository.OrderPatch{
		ShippingAddress: patch.ShippingAddress,
		SelectedCourier: order.SelectedCourier,
		SelectedService: order.SelectedService,
		// Seeded for the same reason as PromoCodeID: the UPDATE writes these
		// columns unconditionally, so a patch that does not mention the courier
		// would null them and AdminShipOrder would then fail on ErrNoCarrierCode.
		CourierCode:        order.CourierCode,
		CourierServiceCode: order.CourierServiceCode,
		IsEstimate:         order.IsEstimate,
		PromoCodeID:        order.PromoCodeID,
		Discount:        order.Discount,
		ShippingCost:    order.ShippingCost,
		Total:           order.Total,
		ProvinceID:      patch.ProvinceID,
		CityID:          patch.CityID,
		DistrictID:      patch.DistrictID,
		KodePos:         patch.KodePos,
	}

	if patch.Courier != "" {
		var weightGrams int
		var itemValue int64
		hasPhysical := false
		for _, item := range order.Items {
			if isPhysicalType(item.ProductType) {
				hasPhysical = true
				weightGrams += item.WeightGrams * item.Qty
				itemValue += int64(item.Jumlah)
			}
		}

		if hasPhysical {
			if patch.ProvinceID == nil || patch.CityID == nil || patch.DistrictID == nil || patch.KodePos == nil {
				return ErrIncompleteAddress
			}
			if err := s.validateAddressHierarchy(ctx, *patch.ProvinceID, *patch.CityID, *patch.DistrictID); err != nil {
				return err
			}

			rates, err := s.GetShippingRates(ctx, ShippingQuoteRequest{
				DestinationPostalCode: *patch.KodePos,
				WeightGrams:           weightGrams,
				ItemValue:             itemValue,
			})
			if err != nil {
				return err
			}
			matched := false
			for _, rate := range rates {
				if strings.EqualFold(rate.Courier, patch.Courier) && strings.EqualFold(rate.Service, patch.Service) {
					repoPatch.ShippingCost = float64(rate.Price)
					repoPatch.IsEstimate = rate.IsEstimate
					repoPatch.CourierCode = &rate.CourierCode
					repoPatch.CourierServiceCode = &rate.ServiceCode
					matched = true
					break
				}
			}
			if !matched {
				return ErrInvalidCourierSelection
			}
			repoPatch.SelectedCourier = patch.Courier
			repoPatch.SelectedService = patch.Service
		}
	} else {
		// A quote is priced for exactly one (destination, weight) pair. If this
		// patch is not itself selecting a courier, any real change to the
		// effective destination — COALESCE(patch, persisted), mirroring the
		// UPDATE's own COALESCE — voids whatever quote was already on the order.
		effective := func(patched, persisted *string) *string {
			if patched != nil {
				return patched
			}
			return persisted
		}
		changed := func(patched, persisted *string) bool {
			eff := effective(patched, persisted)
			if eff == nil && persisted == nil {
				return false
			}
			if eff == nil || persisted == nil {
				return true
			}
			return *eff != *persisted
		}
		if changed(patch.ProvinceID, order.ProvinceID) ||
			changed(patch.CityID, order.CityID) ||
			changed(patch.DistrictID, order.DistrictID) ||
			changed(patch.KodePos, order.KodePos) {
			repoPatch.SelectedCourier = ""
			repoPatch.SelectedService = ""
			repoPatch.CourierCode = nil
			repoPatch.CourierServiceCode = nil
			repoPatch.IsEstimate = false
			repoPatch.ShippingCost = 0
		}
	}

	// JSON null and an absent key are indistinguishable once decoded into *string,
	// so both are treated as "keep" here; "" is the only remove sentinel.
	switch {
	case patch.PromoCode == nil:
		repoPatch.Total = order.Subtotal - repoPatch.Discount + repoPatch.ShippingCost
	case *patch.PromoCode == "":
		repoPatch.PromoCodeID = nil
		repoPatch.Discount = 0
		repoPatch.Total = order.Subtotal - repoPatch.Discount + repoPatch.ShippingCost
	default:
		validation, err := s.ValidatePromo(ctx, *patch.PromoCode, order.Subtotal, repoPatch.ShippingCost)
		if err != nil {
			return err
		}
		repoPatch.PromoCodeID = &validation.PromoID
		repoPatch.Discount = validation.Discount
		repoPatch.Total = validation.Total
	}

	return s.storeRepo.PatchCart(ctx, oID, repoPatch)
}

// freeCheckoutSentinel is cached under the checkout idempotency key when a
// zero-total order settles without the gateway, so a retried checkout replays
// the free result instead of a (non-existent) gateway ref.
const freeCheckoutSentinel = "free"

type CheckoutResult struct {
	GatewayRef       string
	SnapToken        string
	PaymentURL       string
	PaymentExpiresAt time.Time
	// Free is true when a zero-total order was settled directly without the
	// payment gateway — the client should skip the payment page.
	Free bool
}

func fetchCustomerInfo(ctx context.Context, s *Service, userID string) CustomerInfo {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return CustomerInfo{}
	}
	name := user.Name
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}
	return CustomerInfo{Name: name, Email: email, Phone: phone}
}

func buildPaymentRequest(orderID string, order model.Order, customer CustomerInfo) PaymentRequest {
	req := PaymentRequest{
		OrderID:   orderID,
		Amount:    int64(order.Total),
		ExpiresIn: 24 * time.Hour,
		Customer:  customer,
	}

	for _, item := range order.Items {
		cat := "General"
		switch item.ProductType {
		case "book":
			cat = "Book"
		case "merchandise":
			cat = "Merchandise"
		case "medal":
			cat = "Medal"
		case "course":
			cat = "Course"
		}
		req.Items = append(req.Items, ItemDetail{
			ID:       item.ProductID.String(),
			Name:     truncateItemName(item.Name),
			Price:    int64(item.UnitPrice),
			Qty:      int32(item.Qty),
			Category: cat,
		})
	}

	if order.ShippingCost > 0 {
		req.Items = append(req.Items, ItemDetail{
			ID:       "shipping",
			Name:     "Ongkos Kirim",
			Price:    int64(order.ShippingCost),
			Qty:      1,
			Category: "Shipping",
		})
	}

	// Midtrans requires gross_amount == sum(item_details). Discount isn't a line
	// item, so represent it as a negative-priced entry to keep the sum balanced.
	if order.Discount > 0 {
		req.Items = append(req.Items, ItemDetail{
			ID:       "discount",
			Name:     "Diskon",
			Price:    -int64(order.Discount),
			Qty:      1,
			Category: "Discount",
		})
	}

	return req
}

// truncateItemName caps a name at Midtrans's 50-character item_details.name limit.
func truncateItemName(name string) string {
	const midtransItemNameMax = 50
	r := []rune(name)
	if len(r) <= midtransItemNameMax {
		return name
	}
	return string(r[:midtransItemNameMax])
}

type OrderPaidPayload struct {
	OrderID string                 `json:"order_id"`
	Items   []OrderPaidPayloadItem `json:"items"`
}

type OrderPaidPayloadItem struct {
	ProductID   string `json:"product_id"`
	ProductType string `json:"product_type"`
	Qty         int    `json:"qty"`
}

type MidtransNotification struct {
	TransactionStatus string `json:"transaction_status"`
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	GrossAmount       string `json:"gross_amount"`
	StatusCode        string `json:"status_code"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
}

// Checkout deliberately commits the pre-gateway transaction (order status,
// promo redemption) before calling s.payment.CreatePayment. CreatePayment is
// a network call to an external gateway and must not hold a DB transaction
// open for its duration; keeping it outside the tx also means a gateway
// failure never rolls back work that already happened (e.g. stock deduction).
// The cost of the split is that a gateway failure leaves a durable
// payment_pending order whose promo has already been counted — RetryPayment
// is the documented recovery path for that order, and it does not re-count
// the promo, so the redemption is not lost or duplicated on retry.
func (s *Service) Checkout(ctx context.Context, studentID, orderID, key string) (CheckoutResult, error) {
	oID, err := parseUUID(orderID)
	if err != nil {
		return CheckoutResult{}, err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return CheckoutResult{}, err
	}

	cacheKey := "idempotency:checkout:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		if cached == freeCheckoutSentinel {
			return CheckoutResult{Free: true}, nil
		}
		return CheckoutResult{GatewayRef: cached}, nil
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if order.ID.String() == "" {
		return CheckoutResult{}, ErrOrderNotFound
	}
	if order.StudentID != sID {
		return CheckoutResult{}, ErrOrderNotFound
	}
	if order.Status != "cart" {
		return CheckoutResult{}, ErrOrderNotEditable
	}

	for _, item := range order.Items {
		if isPhysicalType(item.ProductType) && (order.ShippingCost <= 0 || order.SelectedCourier == "") {
			return CheckoutResult{}, ErrShippingRequired
		}
	}

	// Re-validate qty at checkout: a row that predates the AddItem/UpdateItemQty
	// guards can still carry qty > 1 on a digital line, and the qty stepper is
	// hidden for digital items so the buyer cannot self-correct it. Catching it
	// here — before BeginTx — stops the wrong price from ever reaching payment.
	for _, item := range order.Items {
		if err := ValidateItemQty(item.ProductType, item.Qty); err != nil {
			return CheckoutResult{}, err
		}
	}

	// Biodata gate: a student registering for an exam for themselves must have
	// complete biodata (school, class, dob). Bulk/admin orders (order_participant
	// rows present) are exempt — those students' biodata is admin-managed.
	hasExam := false
	for _, item := range order.Items {
		if item.ProductType == "exam" {
			hasExam = true
			break
		}
	}
	if hasExam {
		participants, err := s.storeRepo.GetOrderParticipants(ctx, oID)
		if err != nil {
			return CheckoutResult{}, err
		}
		if len(participants) == 0 { // self-purchase
			u, err := s.repo.GetUserByID(ctx, order.StudentID.String())
			if err != nil {
				return CheckoutResult{}, err
			}
			if missing := biodataMissingFields(u); len(missing) > 0 {
				return CheckoutResult{}, &BiodataIncompleteError{Missing: missing}
			}
		}
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return CheckoutResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.CheckoutOrder(ctx, tx, oID); err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			return CheckoutResult{}, ErrInsufficientStock
		}
		return CheckoutResult{}, err
	}

	// Free / zero-total order: skip the payment gateway entirely. Mark it paid
	// and emit OrderPaid in the SAME tx so fulfilment (exam registrations,
	// course access, digital auto-complete) runs exactly as a real settlement.
	if order.Total == 0 {
		if err := s.storeRepo.SetOrderStatus(ctx, tx, oID, "paid", ""); err != nil {
			return CheckoutResult{}, err
		}
		payload := OrderPaidPayload{OrderID: oID.String()}
		for _, item := range order.Items {
			payload.Items = append(payload.Items, OrderPaidPayloadItem{
				ProductID:   item.ProductID.String(),
				ProductType: item.ProductType,
				Qty:         item.Qty,
			})
		}
		if err := s.storeRepo.InsertOutboxEvent(ctx, tx, "order", oID, "OrderPaid", payload); err != nil {
			return CheckoutResult{}, err
		}
		// Promo usage settles inside the same transaction as the order it belongs
		// to. Counting it afterwards can silently lose the increment, and an
		// under-counted promo lets max_uses admit redemptions beyond its limit;
		// there is no gateway round-trip here, so nothing forces it outside.
		if order.PromoCodeID != nil {
			if err := s.storeRepo.IncrementPromoUsesTx(ctx, tx, *order.PromoCodeID); err != nil {
				return CheckoutResult{}, err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return CheckoutResult{}, err
		}
		// Past this point the order is paid and fulfilment is already queued, so
		// the idempotency cache may not fail the call: returning an error here
		// would tell the student the checkout failed, and their retry would hit a
		// non-cart order (ErrOrderNotEditable) forever. A missing sentinel only
		// costs a retry its replay.
		if err := s.rdb.Set(ctx, cacheKey, freeCheckoutSentinel, 24*time.Hour).Err(); err != nil {
			slog.Error("free checkout: cache idempotency sentinel failed after commit",
				"order_id", oID, "error", err)
		}
		return CheckoutResult{Free: true}, nil
	}

	// Promo usage settles in the same pre-gateway transaction as the order,
	// same as the zero-total path above: counting it after CreatePayment
	// returns would lose the increment on every gateway failure, and an
	// under-counted promo lets max_uses admit redemptions past its limit.
	// This means an abandoned or failed checkout still burns a redemption —
	// accepted trade-off (over-counting a ceiling is the safe direction).
	if order.PromoCodeID != nil {
		if err := s.storeRepo.IncrementPromoUsesTx(ctx, tx, *order.PromoCodeID); err != nil {
			return CheckoutResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CheckoutResult{}, err
	}

	customer := fetchCustomerInfo(ctx, s, order.StudentID.String())
	paymentReq := buildPaymentRequest(oID.String(), order, customer)
	paymentResp, err := s.payment.CreatePayment(ctx, paymentReq)
	if err != nil {
		return CheckoutResult{}, err
	}

	if err := s.storeRepo.SetPaymentRef(ctx, oID, paymentResp.GatewayRef, paymentResp.ExpiresAt); err != nil {
		return CheckoutResult{}, err
	}

	result := CheckoutResult{
		GatewayRef:       paymentResp.GatewayRef,
		SnapToken:        paymentResp.SnapToken,
		PaymentURL:       paymentResp.PaymentURL,
		PaymentExpiresAt: paymentResp.ExpiresAt,
	}

	if err := s.rdb.Set(ctx, cacheKey, paymentResp.GatewayRef, 24*time.Hour).Err(); err != nil {
		return CheckoutResult{}, err
	}

	return result, nil
}

func (s *Service) RetryPayment(ctx context.Context, studentID, orderID, key string) (CheckoutResult, error) {
	oID, err := parseUUID(orderID)
	if err != nil {
		return CheckoutResult{}, err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return CheckoutResult{}, err
	}

	cacheKey := "idempotency:retry:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return CheckoutResult{GatewayRef: cached}, nil
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return CheckoutResult{}, err
	}
	if order.ID.String() == "" {
		return CheckoutResult{}, ErrOrderNotFound
	}
	if order.StudentID != sID {
		return CheckoutResult{}, ErrOrderNotFound
	}
	if order.Status != "payment_pending" && order.Status != "payment_expired" {
		return CheckoutResult{}, ErrOrderNotEditable
	}

	customer := fetchCustomerInfo(ctx, s, order.StudentID.String())
	paymentReq := buildPaymentRequest(oID.String(), order, customer)
	paymentResp, err := s.payment.CreatePayment(ctx, paymentReq)
	if err != nil {
		return CheckoutResult{}, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return CheckoutResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.SetPaymentRef(ctx, oID, paymentResp.GatewayRef, paymentResp.ExpiresAt); err != nil {
		return CheckoutResult{}, err
	}

	if err := s.storeRepo.SetOrderStatus(ctx, tx, oID, "payment_pending", ""); err != nil {
		return CheckoutResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CheckoutResult{}, err
	}

	result := CheckoutResult{
		GatewayRef:       paymentResp.GatewayRef,
		SnapToken:        paymentResp.SnapToken,
		PaymentURL:       paymentResp.PaymentURL,
		PaymentExpiresAt: paymentResp.ExpiresAt,
	}

	if err := s.rdb.Set(ctx, cacheKey, paymentResp.GatewayRef, 24*time.Hour).Err(); err != nil {
		return CheckoutResult{}, err
	}

	return result, nil
}

func (s *Service) ListStudentOrders(ctx context.Context, studentID string, cursor string, limit int) ([]model.Order, string, error) {
	sID, err := parseUUID(studentID)
	if err != nil {
		return nil, "", err
	}
	orders, nextCursor, err := s.storeRepo.ListOrders(ctx, repository.OrderFilter{
		StudentID: &sID,
		Cursor:    cursor,
		Limit:     limit,
	})
	if err != nil {
		return nil, "", err
	}

	filtered := make([]model.Order, 0, len(orders))
	for _, o := range orders {
		if o.Status != "cart" {
			filtered = append(filtered, o)
		}
	}
	return filtered, nextCursor, nil
}

func (s *Service) GetStudentOrder(ctx context.Context, studentID, orderID string) (model.Order, error) {
	oID, err := parseUUID(orderID)
	if err != nil {
		return model.Order{}, err
	}
	sID, err := parseUUID(studentID)
	if err != nil {
		return model.Order{}, err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, oID)
	if err != nil {
		return model.Order{}, err
	}
	if order.ID.String() == "" {
		return model.Order{}, ErrOrderNotFound
	}
	if order.StudentID != sID {
		return model.Order{}, ErrOrderNotFound
	}
	return order, nil
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, err
	}
	return id, nil
}

// Admin order methods

// fallbackStudentName is shown in admin views when an order's student row is
// missing (e.g. a deleted account) so the row still renders instead of erroring.
const fallbackStudentName = "Siswa tidak ditemukan"

// attachStudentNames populates StudentName, StudentSchool and StudentGrade on
// every order via one batched call to resolve, regardless of len(orders) —
// FR-35 requires this cost one query for the whole page, not one per order.
// resolve is s.storeRepo.GetBuyersByIDs in production and a counting fake in
// tests.
//
// School and grade ride along in the same lookup because the orders table shows
// them beneath the buyer's name; fetching them separately would reintroduce the
// per-row query FR-35 exists to prevent.
func attachStudentNames(ctx context.Context, orders []model.Order, resolve func(context.Context, []string) (map[string]repository.Buyer, error)) error {
	if len(orders) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(orders))
	ids := make([]string, 0, len(orders))
	for _, o := range orders {
		sid := o.StudentID.String()
		if _, ok := seen[sid]; ok {
			continue
		}
		seen[sid] = struct{}{}
		ids = append(ids, sid)
	}
	buyers, err := resolve(ctx, ids)
	if err != nil {
		return err
	}
	for i := range orders {
		b, ok := buyers[orders[i].StudentID.String()]
		if ok && b.Name != "" {
			orders[i].StudentName = b.Name
		} else {
			orders[i].StudentName = fallbackStudentName
		}
		orders[i].StudentSchool = b.School
		orders[i].StudentGrade = b.Grade
	}
	return nil
}

// AdminListOrders and AdminGetOrder are the only two places StudentName is
// populated — the admin-facing paths. Student-facing paths (ListStudentOrders,
// GetStudentOrder) leave it as the zero value.
func (s *Service) AdminListOrders(ctx context.Context, filter repository.OrderFilter) ([]model.Order, string, error) {
	filter.ExcludeCart = true
	orders, cursor, err := s.storeRepo.ListOrders(ctx, filter)
	if err != nil {
		return nil, "", err
	}
	if err := attachStudentNames(ctx, orders, s.storeRepo.GetBuyersByIDs); err != nil {
		return nil, "", err
	}
	return orders, cursor, nil
}

func (s *Service) AdminGetOrder(ctx context.Context, orderID string) (model.Order, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return model.Order{}, err
	}
	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return model.Order{}, err
	}
	orders := []model.Order{order}
	if err := attachStudentNames(ctx, orders, s.storeRepo.GetBuyersByIDs); err != nil {
		return model.Order{}, err
	}
	return orders[0], nil
}

// paymentProofURLTTL bounds how long a minted payment-proof link stays usable.
// A proof is a bank transfer receipt, so unlike an avatar it is never served
// from the unauthenticated /files/* proxy — the link is minted per request for
// an already-authorised admin and expires, which is what makes a leaked URL
// survivable.
const paymentProofURLTTL = 15 * time.Minute

// PresignPaymentProofURL mints a short-lived read URL for an order's payment
// proof. The caller is gated on orders:write at the route; this resolves the
// key from the order rather than trusting one supplied by the client, so an
// admin cannot use it to read an arbitrary object out of the bucket.
func (s *Service) PresignPaymentProofURL(ctx context.Context, orderID string) (string, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return "", err
	}
	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return "", err
	}
	if order.PaymentProofURL == nil || *order.PaymentProofURL == "" {
		return "", ErrUploadNotFound
	}
	return s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *order.PaymentProofURL, paymentProofURLTTL)
}

// PresignRefundProofURL is the refund-side twin of PresignPaymentProofURL. Kept
// as a separate method rather than taking a caller-supplied kind, so the key
// always comes from a fixed field on the order and the endpoint can never be
// turned into an arbitrary-object reader.
func (s *Service) PresignRefundProofURL(ctx context.Context, orderID string) (string, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return "", err
	}
	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return "", err
	}
	if order.RefundProofURL == nil || *order.RefundProofURL == "" {
		return "", ErrUploadNotFound
	}
	return s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *order.RefundProofURL, paymentProofURLTTL)
}

// markOrderPaidTx flips order to "paid", emits the OrderPaid outbox event,
// and writes an audit row, all inside tx. AdminConfirmOrder and
// AdminReconcileOrder are the only two callers — invariant 5 requires exactly
// one fulfilment path regardless of how an order gets paid.
func (s *Service) markOrderPaidTx(ctx context.Context, tx pgx.Tx, order model.Order, actorID *string, action string, meta map[string]any) error {
	if err := s.storeRepo.SetOrderStatus(ctx, tx, order.ID, "paid", ""); err != nil {
		return err
	}

	payload := OrderPaidPayload{OrderID: order.ID.String()}
	for _, item := range order.Items {
		payload.Items = append(payload.Items, OrderPaidPayloadItem{
			ProductID:   item.ProductID.String(),
			ProductType: item.ProductType,
			Qty:         item.Qty,
		})
	}
	// The aggregate type is explicit as of #69 — orders are no longer the only
	// thing the outbox carries.
	if err := s.storeRepo.InsertOutboxEvent(ctx, tx, "order", order.ID, "OrderPaid", payload); err != nil {
		return err
	}

	return s.storeRepo.InsertAuditLogMeta(ctx, tx, actorID, "order", order.ID.String(), action, meta)
}

func (s *Service) AdminConfirmOrder(ctx context.Context, actorID, orderID, key, paymentProofURL string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	cacheKey := "idempotency:confirm:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return nil
	}

	// The handler validates the shape of the key; only storage can answer
	// whether it names a real upload. Without this an authorised caller could
	// invent a well-formed key and mark an order paid with no evidence behind
	// it, which is exactly what invariant 6 exists to prevent.
	if err := s.requireStoredObject(ctx, paymentProofURL); err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Manual settlement — a human asserting payment arrived, with proof, without
	// gateway confirmation. The proof write, status flip and audit row all share
	// this tx (invariant 6): a confirmed order can never exist without its
	// evidence, so markOrderPaidTx's audit insert failing must roll back the
	// proof write too.
	if err := s.storeRepo.SetOrderManualPayment(ctx, tx, order.ID, paymentProofURL); err != nil {
		return err
	}
	// The OrderPaid insert that used to sit here moved into markOrderPaidTx
	// below, which reconcile now shares (invariant 5: exactly one fulfilment
	// path). Re-adding it here would emit the event twice.
	actor := &actorID
	if err := s.markOrderPaidTx(ctx, tx, order, actor, "order.confirm", map[string]any{
		"manual":            true,
		"payment_proof_url": paymentProofURL,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if err := s.rdb.Set(ctx, cacheKey, "ok", 24*time.Hour).Err(); err != nil {
		return err
	}

	// Push notification (best-effort; non-fatal error)
	// Gate: skip only when explicitly set to "false"
	cfg, _ := s.GetSystemConfig(ctx)
	if purchaseNotifyEnabled(cfg) {
		student, _ := s.storeRepo.GetUserByID(ctx, order.StudentID.String())
		studentName := "Student"
		if student != nil {
			studentName = student.Name
		}
		notif := PurchaseNotification{
			ID:          uuid.New().String(),
			Type:        "order_confirmed",
			OrderID:     order.ID,
			StudentName: studentName,
			Amount:      int64(order.Total),
			CreatedAt:   time.Now(),
			Read:        false,
		}
		_ = s.PushPurchaseNotification(ctx, notif)
	}

	return nil
}

// purchaseNotifyEnabled returns true when the admin_store purchase notification
// should fire. Only "false" disables it; "" (unset) and "true" are enabled.
func purchaseNotifyEnabled(cfg map[string]string) bool {
	return cfg["notify_on_purchase_admin_store"] != "false"
}

func (s *Service) AdminCompleteOrder(ctx context.Context, orderID string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	switch order.Status {
	case "shipped":
		// physical order after delivery — always completable
	case "processing":
		// only completable if no physical items (digital-only orders stuck before worker fix)
		for _, item := range order.Items {
			if isPhysicalType(item.ProductType) {
				return ErrMustShipBeforeComplete
			}
		}
	default:
		return errors.New("order cannot be completed from status: " + order.Status)
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.SetOrderStatus(ctx, tx, id, "completed", ""); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

// refundableStatuses bounds AdminRefundOrder. The admin UI already hides the
// action elsewhere, but the API enforced nothing — a direct call could "refund"
// a cart, an unpaid order, or one already refunded, each of which revokes
// enrollments and clears tracking for money that was never taken.
var refundableStatuses = map[string]bool{
	"paid":       true,
	"processing": true,
	"shipped":    true,
	"completed":  true,
}

// AdminRefundOrder records a refund that a human performed by bank transfer.
// It does NOT move money: PaymentClient has no refund method, so nothing
// reaches the gateway, stock is not restored and no OrderRefunded event is
// emitted — see issue #72 for the real flow. What this does guarantee is that
// the record is honest: the status only moves from a status where money was
// actually taken, and the transfer receipt is stored with it.
func (s *Service) AdminRefundOrder(ctx context.Context, actorID, orderID, refundProofURL string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	// Same reasoning as the confirm path: only storage can say whether a
	// well-formed key names a real upload.
	if err := s.requireStoredObject(ctx, refundProofURL); err != nil {
		return err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	if !refundableStatuses[order.Status] {
		return ErrOrderNotRefundable
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.SetOrderStatus(ctx, tx, id, "cancelled", "refunded"); err != nil {
		return err
	}

	if err := s.storeRepo.RevokeEnrollmentsByOrder(ctx, tx, id); err != nil {
		return err
	}
	if err := s.storeRepo.ClearOrderTracking(ctx, tx, id); err != nil {
		return err
	}
	if err := s.storeRepo.SetOrderRefundProof(ctx, tx, id, refundProofURL); err != nil {
		return err
	}

	actor := &actorID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, actor, "order", id.String(), "order.refund", map[string]any{
		"manual":           true,
		"refund_proof_url": refundProofURL,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Service) AdminReconcileOrder(ctx context.Context, actorID, orderID, key string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	cacheKey := "idempotency:reconcile:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return nil
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}
	// Invariant 7: querying the gateway without a gateway_ref asks about no
	// particular order, so its answer is never grounds to flip this order to
	// paid. Refuse before the gateway is ever called.
	if order.GatewayRef == "" {
		return ErrMissingGatewayRef
	}

	status, err := s.payment.QueryStatus(ctx, order.GatewayRef)
	if err != nil {
		return err
	}

	if status.Paid {
		tx, err := s.storeRepo.BeginTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		actor := &actorID
		if err := s.markOrderPaidTx(ctx, tx, order, actor, "order.reconcile", map[string]any{
			"gateway_ref": order.GatewayRef,
		}); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	if err := s.rdb.Set(ctx, cacheKey, "ok", 24*time.Hour).Err(); err != nil {
		return err
	}

	return nil
}

// Admin promo methods

func (s *Service) AdminListPromoCodes(ctx context.Context) ([]model.PromoCode, error) {
	return s.storeRepo.ListPromoCodes(ctx)
}

func (s *Service) AdminCreatePromoCode(ctx context.Context, p model.PromoCode) (model.PromoCode, error) {
	return s.storeRepo.CreatePromoCode(ctx, p)
}

func (s *Service) AdminUpdatePromoCode(ctx context.Context, id string, maxUses *int, expiresAt *time.Time, isPublic bool) error {
	pID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return s.storeRepo.UpdatePromoCode(ctx, pID, maxUses, expiresAt, isPublic)
}

// ListActivePublicPromos returns is_public promo codes that are still valid
// per ValidatePromo's rules, for display to authenticated students. FR-9/FR-10.
func (s *Service) ListActivePublicPromos(ctx context.Context) ([]model.PromoCode, error) {
	return s.storeRepo.ListActivePublicPromos(ctx)
}

func (s *Service) AdminDeletePromoCode(ctx context.Context, id string) error {
	pID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return s.storeRepo.DeletePromoCode(ctx, pID)
}

// Admin revenue method

func (s *Service) AdminGetRevenue(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	return s.storeRepo.GetRevenue(ctx, from, to)
}

// AdminOrderBuckets counts what needs attention. Which shipment_status spellings
// count as a failure is a domain question, so it is answered here rather than in
// the repository — the same call ListOrders' ?queue=shipment_failed makes.
func (s *Service) AdminOrderBuckets(
	ctx context.Context, filter repository.OrderFilter, monthStart, monthEnd time.Time,
) (repository.OrderBucketCounts, error) {
	filter.ExcludeCart = true
	return s.storeRepo.CountOrdersByBucket(ctx, filter, ShipmentFailureStatusValues(), monthStart, monthEnd)
}

func (s *Service) AdminTopProducts(
	ctx context.Context, from, to time.Time, orderBy string, limit int,
) ([]repository.TopProduct, error) {
	return s.storeRepo.TopProducts(ctx, from, to, orderBy, limit)
}

// Payment webhook handler

func (s *Service) HandlePaymentWebhook(ctx context.Context, payload []byte, signature, key string) error {
	if !s.payment.VerifySignature(payload, signature) {
		return ErrInvalidSignature
	}

	cacheKey := "idempotency:webhook:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return nil
	}

	var notif MidtransNotification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return err
	}

	switch notif.TransactionStatus {
	case "settlement", "capture":
		// existing paid flow
	default:
		slog.Info("midtrans webhook ignored", "transaction_status", notif.TransactionStatus, "order_id", notif.OrderID)
		return nil
	}

	// An order_id that is not a UUID cannot name one of our orders, so this is
	// the same outcome as a UUID we have no row for — report it as not-found
	// rather than letting the parse error surface as a 500. Midtrans's own
	// "test notification" button sends `payment_notif_test_<merchant>_<uuid>`,
	// which lands here; answering 404 keeps that button honestly red instead of
	// teaching us to ignore a genuine notification for an order we lost.
	orderID, err := parseUUID(notif.OrderID)
	if err != nil {
		slog.Info("midtrans webhook: order_id is not a UUID", "order_id", notif.OrderID)
		return ErrOrderNotFound
	}

	order, err := s.storeRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.ID.String() == "" {
		return ErrOrderNotFound
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.InsertWebhookLog(ctx, tx, "payment_success", payload, notif.OrderID); err != nil {
		return err
	}

	if err := s.storeRepo.SetOrderStatus(ctx, tx, orderID, "paid", ""); err != nil {
		return err
	}

	// FR-29: only write payment_method when the gateway names one — an absent
	// payment_type must never blank out a value already on the row.
	if notif.PaymentType != "" {
		if err := s.storeRepo.SetOrderPaymentMethod(ctx, tx, orderID, notif.PaymentType); err != nil {
			return err
		}
	}

	outboxPayload := OrderPaidPayload{OrderID: orderID.String()}
	for _, item := range order.Items {
		outboxPayload.Items = append(outboxPayload.Items, OrderPaidPayloadItem{
			ProductID:   item.ProductID.String(),
			ProductType: item.ProductType,
			Qty:         item.Qty,
		})
	}

	if err := s.storeRepo.InsertOutboxEvent(ctx, tx, "order", orderID, "OrderPaid", outboxPayload); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	if err := s.rdb.Set(ctx, cacheKey, "ok", 24*time.Hour).Err(); err != nil {
		return err
	}

	return nil
}

// isPhysicalType reports whether a product type is shipped physical inventory
// (stock-guarded, ship-before-complete). Book, merchandise, and medal qualify.
func isPhysicalType(t string) bool { return t == "book" || t == "merchandise" || t == "medal" }

// checkTypeRBAC returns ErrForbidden if role is not allowed to manage productType.
func checkTypeRBAC(role, productType string) error {
	switch role {
	case RoleSuperAdmin:
		return nil
	case RoleAdminStore:
		// FR-STORE-ADM-03: admin_store edits price/visibility/promo eligibility on
		// exam-type products too (it cannot touch exam content — tests/questions —
		// which stays under /admin/exams, gated separately by RoleAdminExam).
		if isPhysicalType(productType) || productType == "course" || productType == "exam" {
			return nil
		}
		return ErrForbidden
	case RoleAdminExam:
		// The exam admin owns the whole digital catalogue — exam products and
		// the courses behind them. Physical types stay with admin_store because
		// they carry stock, weight and shipping, which is fulfilment work.
		if productType == "exam" || productType == "course" {
			return nil
		}
		return ErrForbidden
	default:
		return ErrForbidden
	}
}
