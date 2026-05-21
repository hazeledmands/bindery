-- Per-author audiobook root folder (issue #579). Adds a nullable override
-- distinct from root_folder_id: root_folder_id only routes ebooks, so an
-- audiobook-specific column lets an author's audiobooks land in their own
-- directory without an ebook root folder accidentally redirecting them (#421).
-- Nullable: when unset, audiobooks fall back to BINDERY_AUDIOBOOK_DIR.
ALTER TABLE authors ADD COLUMN audiobook_root_folder_id INTEGER;
