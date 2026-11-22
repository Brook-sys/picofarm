package database

import (
	"database/sql"
	"testing"
)

func TestPrinterPricingMigrations(t *testing.T) {
	for _, state := range []string{"fresh", "upgrade", "partial"} {
		t.Run(state, func(t *testing.T) {
			db := openTestDB(t)
			if state != "fresh" {
				if _, err := db.Exec(`INSERT INTO printers (id, name, cost_per_hour_cents, purchase_price_cents) VALUES ('legacy', 'Legacy', 150, 20000)`); err != nil {
					t.Fatal(err)
				}
				if state == "upgrade" {
					if _, err := db.Exec(`ALTER TABLE printers DROP COLUMN pricing_config`); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := db.Exec(`UPDATE printers SET pricing_config = '{"currency":"BRL"}' WHERE id = 'legacy'`); err != nil {
						t.Fatal(err)
					}
				}
			}
			for range 2 {
				if err := RunMigrations(db); err != nil {
					t.Fatal(err)
				}
			}
			if !tableColumns(t, db, "printers")["pricing_config"] {
				t.Fatal("missing pricing_config")
			}
			if state != "fresh" {
				var config sql.NullString
				var hourly, purchase int
				if err := db.QueryRow(`SELECT pricing_config, cost_per_hour_cents, purchase_price_cents FROM printers WHERE id = 'legacy'`).Scan(&config, &hourly, &purchase); err != nil {
					t.Fatal(err)
				}
				if hourly != 150 || purchase != 20000 {
					t.Fatal("legacy values changed")
				}
				if state == "upgrade" && config.Valid {
					t.Fatal("must not infer profile for legacy printer")
				}
				if state == "partial" && config.String != `{"currency":"BRL"}` {
					t.Fatal("existing profile changed")
				}
			}
		})
	}
}
