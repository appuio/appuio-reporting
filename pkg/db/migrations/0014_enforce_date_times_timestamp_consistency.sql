-- Timestamp duplicates the (year, month, day, hour) fields, but is more convenient to use.
-- I'd delete the fields but that would be a pretty breaking change.
-- So we just enforce consistency between the two fields.

ALTER TABLE date_times
  ADD CONSTRAINT date_times_timestamp_check_consistency CHECK (
    (date_times.year || '-' || date_times.month || '-' || date_times.day || ' ' || date_times.hour || ':00:00+00')::timestamptz = date_times.timestamp
  );
