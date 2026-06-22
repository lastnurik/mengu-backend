ALTER TABLE document_analysis ALTER COLUMN event_id SET NOT NULL;
ALTER TABLE document_analysis DROP COLUMN IF EXISTS status;
