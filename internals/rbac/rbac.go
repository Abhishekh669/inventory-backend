package rbac

import "inventory/internals/models"

type Permission string

const (
	AllPermissions Permission = "*"

	//category
	CategoryView   Permission = "category.view"
	CategoryCreate Permission = "category.create"
	CategoryUpdate Permission = "category.update"
	CategoryDelete Permission = "category.delete"

	// products
	ProductView   Permission = "product.view"
	ProductCreate Permission = "product.create"
	ProductUpdate Permission = "product.update"
	ProductDelete Permission = "product.delete"
	CostView      Permission = "product.cost.view" // cost price and margins

	// parties
	CustomerView   Permission = "customer.view"
	CustomerManage Permission = "customer.manage"
	SupplierView   Permission = "supplier.view"
	SupplierManage Permission = "supplier.manage"

	// selling
	SaleCreate Permission = "sale.create"
	SaleView   Permission = "sale.view"
	SaleCancel Permission = "sale.cancel"

	// buying
	PurchaseCreate Permission = "purchase.create"
	PurchaseView   Permission = "purchase.view"
	PurchaseCancel Permission = "purchase.cancel"
	OrderManage    Permission = "purchase_order.manage"

	// money and stock
	PaymentCreate Permission = "payment.create"
	ReturnCreate  Permission = "return.create"
	StockView     Permission = "stock.view"
	StockAdjust   Permission = "stock.adjust"
	ExpenseManage Permission = "expense.manage"

	// reports and admin
	DashboardView Permission = "dashboard.view"
	ReportView    Permission = "report.view"
	ReportProfit  Permission = "report.profit"
	UserManage    Permission = "user.manage"

	//settings
	GetSetting    Permission = "setting.get"
	UpdateSetting Permission = "setting.update"
)

var RolePermissions = map[models.RoleType][]Permission{
	models.RoleAdmin: {AllPermissions},

	models.RoleManager: {
		ProductView, ProductCreate, ProductUpdate,
		CustomerView, CustomerManage, SupplierView, SupplierManage,
		SaleCreate, SaleView,
		PurchaseCreate, PurchaseView, OrderManage,
		PaymentCreate, ReturnCreate,
		StockView, StockAdjust, ExpenseManage,
		DashboardView, ReportView,
		GetSetting, UpdateSetting,
		CategoryCreate, CategoryDelete, CategoryUpdate, CategoryView,
	},

	models.RoleCashier: {
		ProductView,
		CustomerView, CustomerManage,
		SaleCreate, SaleView,
		PaymentCreate,
		StockView, DashboardView,
		GetSetting, UpdateSetting,
	},
}

func HasPermission(role *models.RoleType, permission Permission) bool {
	if role == nil {
		return false
	}

	permissions, ok := RolePermissions[*role]
	if !ok {
		return false
	}

	for _, p := range permissions {
		if p == AllPermissions || p == permission {
			return true
		}
	}

	return false
}
