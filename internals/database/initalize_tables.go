package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var postgressSchemaOrderedTables = []struct {
	Name   string
	Schema string
}{
	{
		Name: "users",
		Schema: `
			CREATE EXTENSION IF NOT EXISTS "pgcrypto";

			DO $$ 
			BEGIN 
				IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gender_type') THEN
					CREATE TYPE gender_type AS ENUM ('male', 'female', 'other');
				END IF;

			END $$;

			CREATE TABLE IF NOT EXISTS users (
  				id        			uuid 		PRIMARY KEY DEFAULT gen_random_uuid(),
  				name           			VARCHAR(100) 	NOT NULL,
  				email          			VARCHAR(150) 	NOT NULL UNIQUE,
				gender		   			gender_type  	NOT NULL DEFAULT 'male',
  				password  				TEXT         	NOT NULL,                 -- store a hash, never plain text
				phone 					VARCHAR(15) 	NOT NULL,
  				role            			VARCHAR(20)  	NOT NULL DEFAULT 'cashier'
  				               				CHECK (role IN ('admin','manager','cashier')),
				last_password_reset_at 	BIGINT 			NOT NULL,
  				last_login_at  			TIMESTAMPTZ,
  				created_at     			TIMESTAMPTZ  	NOT NULL DEFAULT now()
			);

			CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
			CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
		`,
	},

	{
		Name: "setting",
		Schema: `
			CREATE TABLE IF NOT EXISTS setting (
				id 				   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
				name               VARCHAR(150) NOT NULL,
  				address            VARCHAR(250),
  				phone              VARCHAR(30),
  				pan_no             VARCHAR(30),
  				vat_no             VARCHAR(30),
  				default_vat_rate   NUMERIC(5,2)  NOT NULL DEFAULT 13,
  				low_stock_default  NUMERIC(14,3) NOT NULL DEFAULT 0,
  				bill_footer        TEXT,
				created_at     			TIMESTAMPTZ  	NOT NULL DEFAULT now()
			);

			CREATE INDEX IF NOT EXISTS idx_setting_name ON setting(name);
		`,
	},
	{
		Name: "categories",
		Schema: `
			CREATE TABLE IF NOT EXISTS categories (
    			id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    			name        VARCHAR(100) NOT NULL ,
				slug 		VARCHAR(20) NOT NULL UNIQUE,
    			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
			);	
		`,
	},
	{
		Name: "units",
		Schema: `
			CREATE TABLE IF NOT EXISTS units (
  				id       	   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  				name           VARCHAR(30) NOT NULL UNIQUE,           -- pcs, kg, meter, bag ...
  				allow_decimal  BOOLEAN NOT NULL DEFAULT FALSE
			);				
		`,
	},
	{
		Name: "products",
		Schema: `
			
			CREATE TABLE IF NOT EXISTS products (
			    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			    sku                 VARCHAR(50) NOT NULL UNIQUE,
			    name                VARCHAR(200) NOT NULL,

			    category_id         UUID NOT NULL REFERENCES categories(id) ON DELETE SET NULL,
			    unit_id             UUID NOT NULL REFERENCES units(id) ON DELETE RESTRICT,

			    vat_rate            NUMERIC(5,2) NOT NULL DEFAULT 13,

			    cost_price          NUMERIC(14,4) NOT NULL DEFAULT 0,
			    cost_with_vat       NUMERIC(14,4) NOT NULL DEFAULT 0,

			    margin_percent      NUMERIC(6,2) NOT NULL DEFAULT 0,

			    selling_price       NUMERIC(14,2) NOT NULL DEFAULT 0,

			    profit_per_unit     NUMERIC(14,4)
			                        GENERATED ALWAYS AS (selling_price - cost_with_vat) STORED,

			    re_order_level      NUMERIC(14,3) NOT NULL DEFAULT 0,
			    current_stock       NUMERIC(14,3) NOT NULL DEFAULT 0,

			    is_active           BOOLEAN NOT NULL DEFAULT TRUE,

			    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
			    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
			);

			CREATE INDEX  IF NOT EXISTS idx_products_category ON products(category_id);
			CREATE INDEX  IF NOT EXISTS idx_products_unit ON products(unit_id);			
			CREATE INDEX  IF NOT EXISTS idx_products_created_at ON products(created_at);			
			CREATE INDEX  IF NOT EXISTS idx_products_current_stock ON products(current_stock);			
			CREATE INDEX  IF NOT EXISTS idx_products_name ON products(name);			
		`,
	},
	{
		Name: "product_price_history",
		Schema: `

			CREATE TABLE IF NOT EXISTS product_price_history (
			  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			  product_id          uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
			  purchase_item_id    uuid,
			  unit_cost           NUMERIC(14,4) NOT NULL,
			  unit_cost_with_vat  NUMERIC(14,4) NOT NULL,
			  margin_percent      NUMERIC(6,2)  NOT NULL,
			  selling_price       NUMERIC(14,2) NOT NULL,
			  profit_per_unit     NUMERIC(14,4)
			                        GENERATED ALWAYS AS (selling_price - unit_cost_with_vat) STORED,
			  effective_date      DATE NOT NULL DEFAULT CURRENT_DATE,
			  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
			);
			CREATE INDEX  IF NOT EXISTS idx_price_hist_product ON product_price_history(product_id, effective_date);
		`,
	}, {
		Name: "stock_movements",
		Schema: `
        CREATE TABLE IF NOT EXISTS stock_movements (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            product_id      UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
            type            VARCHAR(20) NOT NULL
                                CHECK (type IN (
                                    'opening',
                                    'purchase',
                                    'sale',
                                    'purchase_return',
                                    'sale_return',
                                    'adjustment'
                                )),
            qty_change      NUMERIC(14,3) NOT NULL,
            balance_after   NUMERIC(14,3) NOT NULL,
            ref_type        VARCHAR(30),
            ref_id          UUID,
            note            VARCHAR(250),
            user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
            movement_date   DATE NOT NULL DEFAULT CURRENT_DATE,
            created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

            CHECK (qty_change <> 0),
            CHECK (balance_after >= 0)
        );

        CREATE INDEX IF NOT EXISTS idx_stock_mov_product
            ON stock_movements(product_id, movement_date);

        CREATE INDEX IF NOT EXISTS idx_stock_mov_ref
            ON stock_movements(ref_type, ref_id);
    `,
	},
	{
		Name: "suppliers",
		Schema: `
			 
		CREATE TABLE IF NOT EXISTS suppliers (
		  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		  name             VARCHAR(150) NOT NULL,
		  contact_person   VARCHAR(100),
		  phone            VARCHAR(30),
		  address          VARCHAR(250),
		  pan_no           VARCHAR(30),
		  opening_balance  NUMERIC(14,2) NOT NULL DEFAULT 0,
		  balance          NUMERIC(14,2) NOT NULL DEFAULT 0,   -- amount we owe supplier
		  credit_balance   NUMERIC(14,2) NOT NULL DEFAULT 0,
		  is_active        BOOLEAN NOT NULL DEFAULT TRUE,
		  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_suppliers_pan ON suppliers(pan_no) WHERE pan_no IS NOT NULL;
		`,
	},
	{
		Name: "purchase_orders",
		Schema: `
		CREATE TABLE IF NOT EXISTS purchase_orders (
		  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		  po_no          VARCHAR(30) NOT NULL UNIQUE,
		  purchase_id 	 UUID,
		  supplier_id    UUID NOT NULL REFERENCES suppliers(id),
		  title          VARCHAR(150),
		  order_date     DATE NOT NULL DEFAULT CURRENT_DATE,
		  expected_date  DATE,
		  status         VARCHAR(15) NOT NULL DEFAULT 'draft'
		                 CHECK (status IN ('draft','sent','partial','received','cancelled')),
		  note           TEXT,
		  created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
		  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
		  CHECK (expected_date IS NULL OR expected_date >= order_date)
		);
		CREATE INDEX IF NOT EXISTS idx_po_supplier_date ON purchase_orders(supplier_id, order_date DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_po_date_id       ON purchase_orders(order_date DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_po_status_date   ON purchase_orders(status, order_date DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_po_created_by    ON purchase_orders(created_by);
		CREATE INDEX IF NOT EXISTS idx_po_created_id    ON purchase_orders(created_at DESC, id DESC);


		`,
	},
	{
		Name: "purchase_order_items",
		Schema: `

		CREATE TABLE IF NOT EXISTS purchase_order_items (
		  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		  purchase_order_id   UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
		  product_id          UUID NOT NULL REFERENCES products(id),
		  qty_ordered         NUMERIC(14,3) NOT NULL CHECK (qty_ordered > 0),
		  qty_received        NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (qty_received >= 0),
		  expected_unit_cost  NUMERIC(14,4) CHECK (expected_unit_cost IS NULL OR expected_unit_cost >= 0),
		  UNIQUE (purchase_order_id, product_id)
		);
		CREATE INDEX IF NOT EXISTS idx_poi_product ON purchase_order_items(product_id);

		`,
	},
	{
		Name: "purchase_order_counter",
		Schema: `
		CREATE TABLE IF NOT EXISTS po_counters (
		  po_year  INT PRIMARY KEY,
		  last_no  INT NOT NULL
		);
		`,
	},
	{
		Name: "purchases",
		Schema: `
        CREATE TABLE IF NOT EXISTS purchases (
            id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            supplier_id         UUID NOT NULL REFERENCES suppliers(id),
            purchase_order_id   UUID REFERENCES purchase_orders(id),

            supplier_bill_no    VARCHAR(50) NOT NULL,
            purchase_date       DATE NOT NULL DEFAULT CURRENT_DATE,

            excise_duty         NUMERIC(14,2) NOT NULL DEFAULT 0,
            special_discount    NUMERIC(14,2) NOT NULL DEFAULT 0,
            discount            NUMERIC(14,2) NOT NULL DEFAULT 0,
            vat_rate            NUMERIC(5,2) NOT NULL DEFAULT 13,
            freight_charge      NUMERIC(14,2) NOT NULL DEFAULT 0,

            gross_amount        NUMERIC(14,2) NOT NULL DEFAULT 0,
            taxable_amount      NUMERIC(14,2) NOT NULL DEFAULT 0,
            vat_amount          NUMERIC(14,2) NOT NULL DEFAULT 0,
            total               NUMERIC(14,2) NOT NULL DEFAULT 0,

            paid_amount         NUMERIC(14,2) NOT NULL DEFAULT 0,
            returned_amount     NUMERIC(14,2) NOT NULL DEFAULT 0,

            due_amount          NUMERIC(14,2)
                                GENERATED ALWAYS AS (
                                    total - paid_amount - returned_amount
                                ) STORED,

            payment_status      VARCHAR(10) NOT NULL DEFAULT 'unpaid'
                                CHECK (payment_status IN ('unpaid', 'partial', 'paid')),

            due_date            DATE,

            status              VARCHAR(10) NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'posted', 'cancelled')),

            note                TEXT,

            created_by          UUID REFERENCES users(id) ON DELETE SET NULL,
            created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

            UNIQUE (supplier_id, supplier_bill_no),

            CHECK (excise_duty >= 0),
            CHECK (special_discount >= 0),
            CHECK (discount >= 0),
            CHECK (freight_charge >= 0),
            CHECK (paid_amount >= 0),
            CHECK (returned_amount >= 0),
            CHECK (total >= 0)
        );

        CREATE INDEX IF NOT EXISTS idx_purchases_supplier
        ON purchases(supplier_id);

        CREATE INDEX IF NOT EXISTS idx_purchases_date
        ON purchases(purchase_date);

        CREATE INDEX IF NOT EXISTS idx_purchases_status
        ON purchases(status);

        CREATE INDEX IF NOT EXISTS idx_purchase_returns_supplier_date
        ON purchase_returns(supplier_id, return_date DESC, created_at DESC, id DESC);
    
		
    `,
	},
	{
		Name: "purchase_items",
		Schema: `
        CREATE TABLE IF NOT EXISTS purchase_items (
            id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            purchase_id             UUID NOT NULL
                                    REFERENCES purchases(id) ON DELETE CASCADE,

            product_id              UUID NOT NULL REFERENCES products(id),
            poi_id                  UUID REFERENCES purchase_order_items(id),

            qty                     NUMERIC(14,3) NOT NULL CHECK (qty > 0),
            rate                    NUMERIC(14,4) NOT NULL CHECK (rate >= 0),

            vat_amount              NUMERIC(14,4) NOT NULL DEFAULT 0,

            unit_cost               NUMERIC(14,4) NOT NULL DEFAULT 0,
            unit_cost_with_vat      NUMERIC(14,4) NOT NULL DEFAULT 0

        );

        CREATE INDEX IF NOT EXISTS idx_purchase_items_purchase
        ON purchase_items(purchase_id);

        CREATE INDEX IF NOT EXISTS idx_purchase_items_product
        ON purchase_items(product_id);
    `,
	},
	{
		Name: "purchase_returns",
		Schema: `
        CREATE TABLE IF NOT EXISTS purchase_returns (
            id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            return_no           VARCHAR(30) NOT NULL UNIQUE,

            purchase_id         UUID NOT NULL REFERENCES purchases(id),
            supplier_id         UUID NOT NULL REFERENCES suppliers(id),

            return_date         DATE NOT NULL DEFAULT CURRENT_DATE,

            total               NUMERIC(14,2) NOT NULL DEFAULT 0,

            note                TEXT,

            created_by          UUID REFERENCES users(id) ON DELETE SET NULL,
            created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

            CHECK (total >= 0)
        );

        CREATE INDEX IF NOT EXISTS idx_purchase_returns_purchase
        ON purchase_returns(purchase_id);

        CREATE INDEX IF NOT EXISTS idx_purchase_returns_supplier
        ON purchase_returns(supplier_id);

        CREATE INDEX IF NOT EXISTS idx_purchase_returns_supplier_date
        ON purchase_returns(supplier_id, return_date DESC, created_at DESC, id DESC);
    `,
	},
	{
		Name: "purchase_return_items",
		Schema: `
        CREATE TABLE IF NOT EXISTS purchase_return_items (
            id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            purchase_return_id  UUID NOT NULL
                                REFERENCES purchase_returns(id) ON DELETE CASCADE,

            purchase_item_id    UUID NOT NULL REFERENCES purchase_items(id),

            product_id          UUID NOT NULL REFERENCES products(id),

            qty                 NUMERIC(14,3) NOT NULL CHECK (qty > 0),

            amount              NUMERIC(14,2) NOT NULL CHECK (amount >= 0),

            reason              VARCHAR(250)
        );

        CREATE INDEX IF NOT EXISTS idx_purchase_return_items_purchase
        ON purchase_return_items(purchase_return_id);

        CREATE INDEX IF NOT EXISTS idx_purchase_return_items_item
        ON purchase_return_items(purchase_item_id);
    `,
	},
	{
		Name: "supplier_payments",
		Schema: `
        CREATE TABLE IF NOT EXISTS supplier_payments (
            id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            supplier_id     UUID NOT NULL REFERENCES suppliers(id),

            amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),

            payment_date    DATE NOT NULL DEFAULT CURRENT_DATE,

            payment_method  VARCHAR(20) NOT NULL DEFAULT 'cash'
                            CHECK (payment_method IN (
                                'cash', 'bank', 'cheque', 'online'
                            )),

            payment_type    VARCHAR(20) NOT NULL DEFAULT 'regular'
                            CHECK (payment_type IN ('advance', 'regular')),

            note            TEXT,

            created_by      UUID REFERENCES users(id) ON DELETE SET NULL,

            created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
        );

        CREATE INDEX IF NOT EXISTS idx_supplier_payments_supplier
        ON supplier_payments(supplier_id);

        CREATE INDEX IF NOT EXISTS idx_supplier_payments_date
        ON supplier_payments(payment_date);

        CREATE INDEX IF NOT EXISTS idx_supplier_payments_supplier_date
        ON supplier_payments(supplier_id, payment_date DESC, created_at DESC);
    `,
	},
	{
		Name: "purchase_payment_allocations",
		Schema: `
        CREATE TABLE IF NOT EXISTS purchase_payment_allocations (
            id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),

            supplier_payment_id UUID NOT NULL
                                REFERENCES supplier_payments(id) ON DELETE RESTRICT,

            purchase_id         UUID NOT NULL
                                REFERENCES purchases(id) ON DELETE RESTRICT,

            amount              NUMERIC(14,2) NOT NULL
                                CHECK (amount > 0),

            created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

            UNIQUE (supplier_payment_id, purchase_id)
        );

        CREATE INDEX IF NOT EXISTS idx_payment_allocations_purchase
        ON purchase_payment_allocations(purchase_id);

        CREATE INDEX IF NOT EXISTS idx_payment_allocations_payment
        ON purchase_payment_allocations(supplier_payment_id);
    `,
	},
	{
		Name: "supplier_financial_summaries",
		Schema: `
        CREATE TABLE IF NOT EXISTS supplier_financial_summaries (
            supplier_id       UUID PRIMARY KEY
                              REFERENCES suppliers(id) ON DELETE CASCADE,

            total_purchases   NUMERIC(16,2) NOT NULL DEFAULT 0
                              CHECK (total_purchases >= 0),

            total_payments    NUMERIC(16,2) NOT NULL DEFAULT 0
                              CHECK (total_payments >= 0),

            total_returns     NUMERIC(16,2) NOT NULL DEFAULT 0
                              CHECK (total_returns >= 0),
			unallocated_payments NUMERIC(16,2) NOT NULL DEFAULT 0
                CHECK (unallocated_payments >= 0),


            updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
        );
    `,
	},
}

func CreatePostgresTables(ctx context.Context, postgresPool *pgxpool.Pool) error {
	for _, t := range postgressSchemaOrderedTables {
		if err := ValidateTableName(t.Name); err != nil {
			return fmt.Errorf("invalid table name: %s", t.Name)
		}

		if _, err := postgresPool.Exec(ctx, t.Schema); err != nil {
			return fmt.Errorf("failed to create table %s: %w", t.Name, err)
		}
	}

	if err := seedSetting(ctx, postgresPool); err != nil {
		return fmt.Errorf("failed to inventory information: %w", err)
	}

	return nil
}

func ValidateTableName(tableName string) error {
	if tableName == "" {
		return fmt.Errorf("table name cannot be empty")
	}

	if len(tableName) > 63 {
		return fmt.Errorf("table name too long (max 63 characters)")
	}

	if !pgIdentifierRegex.MatchString(tableName) {
		return fmt.Errorf("invalid table name: must start with letter or underscore, contain only letters, numbers, and underscores")
	}

	return nil
}

func seedSetting(ctx context.Context, db *pgxpool.Pool) error {
	query := `
		INSERT INTO setting (
			name,
			address,
			phone,
			pan_no,
			vat_no,
			default_vat_rate,
			low_stock_default,
			bill_footer
		)
		SELECT
			'My Hardware Store',
			'Kathmandu, Nepal',
			NULL,
			NULL,
			NULL,
			13,
			0,
			'Thank you for your purchase!'
		WHERE NOT EXISTS (
			SELECT 1 FROM setting
		);
	`

	_, err := db.Exec(ctx, query)
	return err
}
