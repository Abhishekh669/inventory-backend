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
