package internal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Storage handles persistence of image hashes and duplicate groups
type Storage struct {
	db     *sql.DB
	dbPath string
}

// NewStorage creates a new Storage
func NewStorage(dbPath string) (*Storage, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Storage{db: db, dbPath: dbPath}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// init creates the database schema
func (s *Storage) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		hash INTEGER NOT NULL,
		file_hash TEXT DEFAULT '',
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		format TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		mod_time DATETIME NOT NULL,
		has_exif INTEGER DEFAULT 0,
		score REAL NOT NULL,
		group_id INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_images_hash ON images(hash);
	CREATE INDEX IF NOT EXISTS idx_images_group_id ON images(group_id);
	CREATE INDEX IF NOT EXISTS idx_images_path ON images(path);

	CREATE TABLE IF NOT EXISTS scan_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		folder TEXT NOT NULL,
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		total_images INTEGER NOT NULL,
		total_groups INTEGER NOT NULL,
		total_duplicates INTEGER NOT NULL
	);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing databases
	if err := s.migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// migrate handles schema migrations for existing databases
func (s *Storage) migrate() error {
	// Check if file_hash column exists
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('images') WHERE name='file_hash'
	`).Scan(&count)
	if err != nil {
		return err
	}

	// Add file_hash column if it doesn't exist
	if count == 0 {
		_, err = s.db.Exec(`ALTER TABLE images ADD COLUMN file_hash TEXT DEFAULT ''`)
		if err != nil {
			return err
		}
	}

	// Create index on file_hash (will be no-op if already exists)
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_images_file_hash ON images(file_hash)`)
	if err != nil {
		return err
	}

	return nil
}

// Close closes the database connection
func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveImages saves or updates multiple images
func (s *Storage) SaveImages(images []*ImageInfo) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO images (path, hash, file_hash, width, height, format, file_size, mod_time, has_exif, score, group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, img := range images {
		// Cast uint64 to int64 for SQLite compatibility
		hashInt := int64(img.Hash)
		hasExifInt := 0
		if img.HasExif {
			hasExifInt = 1
		}
		_, err := stmt.Exec(
			img.Path,
			hashInt,
			img.FileHash,
			img.Width,
			img.Height,
			img.Format,
			img.FileSize,
			img.ModTime,
			hasExifInt,
			img.Score,
			img.GroupID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert image %s: %w", img.Path, err)
		}
	}

	return tx.Commit()
}

// GetAllImages returns all stored images
func (s *Storage) GetAllImages() ([]*ImageInfo, error) {
	rows, err := s.db.Query(`
		SELECT id, path, hash, file_hash, width, height, format, file_size, mod_time, has_exif, score, group_id
		FROM images
		ORDER BY path
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	defer rows.Close()

	var images []*ImageInfo
	for rows.Next() {
		img := &ImageInfo{}
		var modTime string
		var hashInt int64
		var hasExifInt int
		var fileHash sql.NullString
		err := rows.Scan(
			&img.ID,
			&img.Path,
			&hashInt,
			&fileHash,
			&img.Width,
			&img.Height,
			&img.Format,
			&img.FileSize,
			&modTime,
			&hasExifInt,
			&img.Score,
			&img.GroupID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		img.Hash = uint64(hashInt)
		img.FileHash = fileHash.String
		img.HasExif = hasExifInt == 1
		img.ModTime, _ = time.Parse("2006-01-02 15:04:05", modTime)
		images = append(images, img)
	}

	return images, nil
}

// UpdateGroups updates group IDs for images
func (s *Storage) UpdateGroups(groups []*DuplicateGroup) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Reset all group IDs
	_, err = tx.Exec("UPDATE images SET group_id = 0")
	if err != nil {
		return fmt.Errorf("failed to reset groups: %w", err)
	}

	stmt, err := tx.Prepare("UPDATE images SET group_id = ? WHERE path = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, group := range groups {
		for _, img := range group.Images {
			_, err := stmt.Exec(group.ID, img.Path)
			if err != nil {
				return fmt.Errorf("failed to update group for %s: %w", img.Path, err)
			}
		}
	}

	return tx.Commit()
}

// GetImagesByGroupID returns images in a specific group
func (s *Storage) GetImagesByGroupID(groupID int) ([]*ImageInfo, error) {
	rows, err := s.db.Query(`
		SELECT id, path, hash, file_hash, width, height, format, file_size, mod_time, has_exif, score, group_id
		FROM images
		WHERE group_id = ?
		ORDER BY score DESC
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query images: %w", err)
	}
	defer rows.Close()

	var images []*ImageInfo
	for rows.Next() {
		img := &ImageInfo{}
		var modTime string
		var hashInt int64
		var hasExifInt int
		var fileHash sql.NullString
		err := rows.Scan(
			&img.ID,
			&img.Path,
			&hashInt,
			&fileHash,
			&img.Width,
			&img.Height,
			&img.Format,
			&img.FileSize,
			&modTime,
			&hasExifInt,
			&img.Score,
			&img.GroupID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		img.Hash = uint64(hashInt)
		img.FileHash = fileHash.String
		img.HasExif = hasExifInt == 1
		img.ModTime, _ = time.Parse("2006-01-02 15:04:05", modTime)
		images = append(images, img)
	}

	return images, nil
}

// DeleteImage removes an image from the database
func (s *Storage) DeleteImage(path string) error {
	_, err := s.db.Exec("DELETE FROM images WHERE path = ?", path)
	return err
}

// RecordScan records a scan in history
func (s *Storage) RecordScan(folder string, totalImages, totalGroups, totalDuplicates int) error {
	_, err := s.db.Exec(`
		INSERT INTO scan_history (folder, total_images, total_groups, total_duplicates)
		VALUES (?, ?, ?, ?)
	`, folder, totalImages, totalGroups, totalDuplicates)
	return err
}

// GetGroupCount returns the number of duplicate groups
func (s *Storage) GetGroupCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(DISTINCT group_id) FROM images WHERE group_id > 0").Scan(&count)
	return count, err
}

// GetDuplicateGroups returns all duplicate groups with their images
func (s *Storage) GetDuplicateGroups() ([]*DuplicateGroup, error) {
	// Get distinct group IDs
	rows, err := s.db.Query("SELECT DISTINCT group_id FROM images WHERE group_id > 0 ORDER BY group_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, id)
	}

	// Build groups
	var groups []*DuplicateGroup
	for _, id := range groupIDs {
		images, err := s.GetImagesByGroupID(id)
		if err != nil {
			return nil, err
		}

		if len(images) < 2 {
			continue
		}

		group := &DuplicateGroup{
			ID:     id,
			Images: images,
			Keep:   images[0], // Already sorted by score DESC
			Remove: images[1:],
		}
		groups = append(groups, group)
	}

	return groups, nil
}
