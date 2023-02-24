ALTER TABLE tenants
  ADD COLUMN during tstzrange NOT NULL DEFAULT '[-infinity,infinity)',
  ADD CONSTRAINT tenants_source_during_non_overlapping EXCLUDE USING GIST (source WITH =, during WITH &&),
  ADD CONSTRAINT tenants_during_lower_not_null_ck CHECK (lower(during) IS NOT NULL),
  ADD CONSTRAINT tenants_during_upper_not_null_ck CHECK (upper(during) IS NOT NULL),
  DROP CONSTRAINT tenants_source_key;
