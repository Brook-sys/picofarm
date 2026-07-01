-- fluidd_url: optional Fluidd dashboard URL for Moonraker printers
ALTER TABLE printers ADD COLUMN fluidd_url TEXT DEFAULT '';
