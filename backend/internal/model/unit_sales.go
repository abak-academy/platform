package model

type UnitSalesFigures struct {
	PaidOrders   int `json:"paid_orders"`
	RefundOrders int `json:"refund_orders"`
	PaidUnits    int `json:"paid_units"`
	RefundUnits  int `json:"refund_units"`
	NetUnits     int `json:"net_units"`
}

type UnitSalesProduct struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	UnitSalesFigures
}

type UnitSalesType struct {
	Type string `json:"type"`
	UnitSalesFigures
	Products []UnitSalesProduct `json:"products"`
}

type UnitSalesResponse struct {
	From  *string         `json:"from"`
	To    *string         `json:"to"`
	Types []UnitSalesType `json:"types"`
}
