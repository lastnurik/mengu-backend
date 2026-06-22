ALTER TABLE document_analysis ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'uploaded';
ALTER TABLE document_analysis ALTER COLUMN event_id DROP NOT NULL;
