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
			  purchase_item_id    uuid,                           -- FK added after purchase_items exists
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
		  is_active        BOOLEAN NOT NULL DEFAULT TRUE,
		  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
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
