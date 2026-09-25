-- Original client filename for blob downloads and attachment labels.

ALTER TABLE upload_sessions ADD COLUMN original_filename TEXT;
ALTER TABLE blobs ADD COLUMN original_filename TEXT;
