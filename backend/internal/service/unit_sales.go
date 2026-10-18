package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
)

func (s *Service) AdminUnitSales(ctx context.Context, from, to *time.Time) ([]model.UnitSalesType, error) {
	rows, err := s.storeRepo.UnitSales(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("unit sales: %w", err)
	}
	types := make([]model.UnitSalesType, 5)
	for i, typ := range []string{"book", "merchandise", "medal", "course", "exam"} {
		types[i] = model.UnitSalesType{Type: typ, Products: []model.UnitSalesProduct{}}
		for _, row := range rows {
			if row.Type != typ {
				continue
			}
			figures := row.UnitSalesFigures
			figures.NetUnits = figures.PaidUnits - figures.RefundUnits
			if row.ProductID == nil {
				types[i].UnitSalesFigures = figures
			} else {
				types[i].Products = append(types[i].Products, model.UnitSalesProduct{ProductID: *row.ProductID, Name: row.Name, UnitSalesFigures: figures})
			}
		}
		sort.Slice(types[i].Products, func(a, b int) bool {
			x, y := types[i].Products[a], types[i].Products[b]
			if x.NetUnits != y.NetUnits {
				return x.NetUnits > y.NetUnits
			}
			if x.Name != y.Name {
				return x.Name < y.Name
			}
			return x.ProductID < y.ProductID
		})
	}
	return types, nil
}

func UnitSalesCSV(products []model.UnitSalesProduct) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"Produk", "Order dibayar", "Order refund", "Unit dibayar", "Unit refund", "Unit neto"}); err != nil {
		return nil, err
	}
	for _, p := range products {
		if err := writer.Write([]string{guardCSVName(p.Name), strconv.Itoa(p.PaidOrders), strconv.Itoa(p.RefundOrders), strconv.Itoa(p.PaidUnits), strconv.Itoa(p.RefundUnits), strconv.Itoa(p.NetUnits)}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func guardCSVName(name string) string {
	if name != "" && strings.ContainsRune("=+-@\t\r", rune(name[0])) {
		return "'" + name
	}
	return name
}
