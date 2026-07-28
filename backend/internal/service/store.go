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

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// biodataComplete reports whether a student has the biodata required to register
// for an exam: a school (listed or unlisted), a class/grade, and a date of birth.
func biodataComplete(u *model.User) bool {
	if u == nil {
		return false
	}
	hasSchool := (u.SchoolID != nil && *u.SchoolID != "") ||
		(u.UnlistedSchoolName != nil && *u.UnlistedSchoolName != "")
	return hasSchool && u.Grade != nil && u.DOB != nil
}

type PromoValidation struct {
	PromoID  uuid.UUID
	Code     string
	Discount float64
	Total    float64
}

func (s *Service) ListProducts(ctx context.Context, filter repository.ProductFilter, role string) ([]model.Product, string, error) {
	switch role {
	case RoleSuperAdmin:
		// no filter restrictions
	case RoleAdminStore:
		// no filter restrictions — manages book, course, exam
	default: // student, admin_exam, or ""
		filter.VisibleOnly = true
		filter.Status = "published"
	}
	return s.storeRepo.ListProducts(ctx, filter)
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

	return resolveShippingRates(rates, clientErr, flatRate)
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
		hasPhysical := false
		for _, item := range order.Items {
			if isPhysicalType(item.ProductType) {
				hasPhysical = true
				weightGrams += item.WeightGrams * item.Qty
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
			})
			if err != nil {
				return err
			}
			matched := false
			for _, rate := range rates {
				if strings.EqualFold(rate.Courier, patch.Courier) && strings.EqualFold(rate.Service, patch.Service) {
					repoPatch.ShippingCost = float64(rate.Price)
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
	}

	if patch.PromoCode != nil && *patch.PromoCode != "" {
		validation, err := s.ValidatePromo(ctx, *patch.PromoCode, order.Subtotal, repoPatch.ShippingCost)
		if err != nil {
			return err
		}
		repoPatch.PromoCodeID = &validation.PromoID
		repoPatch.Discount = validation.Discount
		repoPatch.Total = validation.Total
	} else {
		repoPatch.Total = order.Subtotal - repoPatch.Discount + repoPatch.ShippingCost
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
}

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
		if isPhysicalType(item.ProductType) && order.ShippingCost <= 0 {
			return CheckoutResult{}, ErrShippingRequired
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
			if !biodataComplete(u) {
				return CheckoutResult{}, ErrBiodataIncomplete
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
		if err := s.storeRepo.InsertOutboxEvent(ctx, tx, oID, "OrderPaid", payload); err != nil {
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

	if order.PromoCodeID != nil {
		if err := s.storeRepo.IncrementPromoUses(ctx, *order.PromoCodeID); err != nil {
			return CheckoutResult{}, err
		}
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

func (s *Service) AdminListOrders(ctx context.Context, filter repository.OrderFilter) ([]model.Order, string, error) {
	filter.ExcludeCart = true
	return s.storeRepo.ListOrders(ctx, filter)
}

func (s *Service) AdminGetOrder(ctx context.Context, orderID string) (model.Order, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return model.Order{}, err
	}
	return s.storeRepo.GetOrderByID(ctx, id)
}

func (s *Service) AdminConfirmOrder(ctx context.Context, actorID, orderID, key string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	cacheKey := "idempotency:confirm:" + key
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

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.storeRepo.SetOrderStatus(ctx, tx, id, "paid", ""); err != nil {
		return err
	}

	payload := OrderPaidPayload{OrderID: id.String()}
	for _, item := range order.Items {
		payload.Items = append(payload.Items, OrderPaidPayloadItem{
			ProductID:   item.ProductID.String(),
			ProductType: item.ProductType,
			Qty:         item.Qty,
		})
	}

	if err := s.storeRepo.InsertOutboxEvent(ctx, tx, id, "OrderPaid", payload); err != nil {
		return err
	}

	// Manual settlement — a human asserting payment arrived without gateway proof.
	// Record who/when in the same tx as the status flip so they commit atomically.
	actor := &actorID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, actor, "order", id.String(), "order.confirm", map[string]any{
		"manual": true,
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
			Amount:      int64(order.Total * 100),
			CreatedAt:   time.Now(),
			Read:        false,
		}
		_ = s.PushPurchaseNotification(ctx, RoleAdminStore, notif)
	}

	return nil
}

// purchaseNotifyEnabled returns true when the admin_store purchase notification
// should fire. Only "false" disables it; "" (unset) and "true" are enabled.
func purchaseNotifyEnabled(cfg map[string]string) bool {
	return cfg["notify_on_purchase_admin_store"] != "false"
}

func (s *Service) AdminShipOrder(ctx context.Context, orderID, trackingNumber string) error {
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

	if order.Status != "paid" && order.Status != "processing" {
		return errors.New("order not in shippable status")
	}

	return s.storeRepo.SetShipped(ctx, id, trackingNumber)
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

func (s *Service) AdminRefundOrder(ctx context.Context, actorID, orderID string) error {
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

	actor := &actorID
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, actor, "order", id.String(), "order.refund", map[string]any{
		"manual": true,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Service) AdminReconcileOrder(ctx context.Context, orderID, key string) error {
	id, err := parseUUID(orderID)
	if err != nil {
		return err
	}

	cacheKey := "idempotency:reconcile:" + key
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		return nil
	}

	status, err := s.payment.QueryStatus(ctx, "")
	if err != nil {
		return err
	}

	if status.Paid {
		if err := s.storeRepo.SetOrderStatus(ctx, nil, id, "paid", ""); err != nil {
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

func (s *Service) AdminUpdatePromoCode(ctx context.Context, id string, maxUses *int, expiresAt *time.Time) error {
	pID, err := parseUUID(id)
	if err != nil {
		return err
	}
	return s.storeRepo.UpdatePromoCode(ctx, pID, maxUses, expiresAt)
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

	outboxPayload := OrderPaidPayload{OrderID: orderID.String()}
	for _, item := range order.Items {
		outboxPayload.Items = append(outboxPayload.Items, OrderPaidPayloadItem{
			ProductID:   item.ProductID.String(),
			ProductType: item.ProductType,
			Qty:         item.Qty,
		})
	}

	if err := s.storeRepo.InsertOutboxEvent(ctx, tx, orderID, "OrderPaid", outboxPayload); err != nil {
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
		if productType == "exam" {
			return nil
		}
		return ErrForbidden
	default:
		return ErrForbidden
	}
}
