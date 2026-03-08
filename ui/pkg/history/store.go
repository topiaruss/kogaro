package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ScanRecord represents a single scan run.
type ScanRecord struct {
	ID            uint         `json:"id" gorm:"primaryKey"`
	Context       string       `json:"context" gorm:"index;not null"`
	ScanTime      time.Time    `json:"scanTime" gorm:"not null"`
	NodeCount     int          `json:"nodeCount"`
	EdgeCount     int          `json:"edgeCount"`
	IncidentCount int          `json:"incidentCount"`
	ErrorCount    int          `json:"errorCount"`
	Errors []ErrorRecord `json:"errors,omitempty"`
}

// ErrorRecord represents a single validation error within a scan.
type ErrorRecord struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	ScanRecordID   uint   `json:"scanId" gorm:"index;not null"`
	ErrorCode    string `json:"errorCode" gorm:"not null"`
	ResourceType string `json:"resourceType" gorm:"not null"`
	ResourceName string `json:"resourceName" gorm:"not null"`
	Namespace    string `json:"namespace" gorm:"default:''"`
	Message      string `json:"message" gorm:"not null"`
	Severity     string `json:"severity" gorm:"default:'error'"`
}

// DiagnosticRecord persists a diagnostic run result.
type DiagnosticRecord struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ErrorCode    string    `json:"errorCode" gorm:"not null"`
	ResourceType string    `json:"resourceType" gorm:"not null"`
	ResourceName string    `json:"resourceName" gorm:"not null"`
	Namespace    string    `json:"namespace" gorm:"default:''"`
	FindingsJSON string    `json:"-" gorm:"type:text"`
	RanAt        time.Time `json:"ranAt" gorm:"not null"`
}

// ScanDiff shows what changed between two scans.
type ScanDiff struct {
	OlderScan   ScanRecord    `json:"olderScan"`
	NewerScan   ScanRecord    `json:"newerScan"`
	NewErrors   []ErrorRecord `json:"newErrors"`
	FixedErrors []ErrorRecord `json:"fixedErrors"`
	Unchanged   int           `json:"unchanged"`
}

// Store persists scan results in SQLite for change tracking.
type Store struct {
	db *gorm.DB
}

// DefaultPath returns ~/.kogaro/history.db
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kogaro")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.db"), nil
}

// Open creates or opens the SQLite database.
func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path+"?_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.AutoMigrate(&ScanRecord{}, &ErrorRecord{}, &DiagnosticRecord{}); err != nil {
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return &Store{db: db}, nil
}

// SaveScan persists a FaultGraph scan result.
func (s *Store) SaveScan(kubeContext string, fg *graph.FaultGraph) (uint, error) {
	var errors []ErrorRecord
	for _, inc := range fg.Incidents {
		for _, e := range inc.Errors {
			errors = append(errors, ErrorRecord{
				ErrorCode:    e.ErrorCode,
				ResourceType: e.ResourceType,
				ResourceName: e.ResourceName,
				Namespace:    e.Namespace,
				Message:      e.Message,
				Severity:     e.Severity,
			})
		}
	}

	record := ScanRecord{
		Context:       kubeContext,
		ScanTime:      fg.ScanTime,
		NodeCount:     len(fg.Nodes),
		EdgeCount:     len(fg.Edges),
		IncidentCount: len(fg.Incidents),
		ErrorCount:    len(errors),
		Errors:        errors,
	}

	if err := s.db.Create(&record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

// GetScanHistory returns recent scans, most recent first.
func (s *Store) GetScanHistory(kubeContext string, limit int) ([]ScanRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	var records []ScanRecord
	err := s.db.Where("context = ?", kubeContext).
		Order("scan_time desc").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetScanErrors returns errors for a specific scan.
func (s *Store) GetScanErrors(scanID uint) ([]ErrorRecord, error) {
	var records []ErrorRecord
	err := s.db.Where("scan_record_id = ?", scanID).
		Order("error_code, resource_name").
		Find(&records).Error
	return records, err
}

// DiffScans compares two scans by error fingerprint (code+resource+namespace).
func (s *Store) DiffScans(olderScanID, newerScanID uint) (*ScanDiff, error) {
	var olderScan, newerScan ScanRecord
	if err := s.db.First(&olderScan, olderScanID).Error; err != nil {
		return nil, fmt.Errorf("older scan %d: %w", olderScanID, err)
	}
	if err := s.db.First(&newerScan, newerScanID).Error; err != nil {
		return nil, fmt.Errorf("newer scan %d: %w", newerScanID, err)
	}

	olderErrors, err := s.GetScanErrors(olderScanID)
	if err != nil {
		return nil, err
	}
	newerErrors, err := s.GetScanErrors(newerScanID)
	if err != nil {
		return nil, err
	}

	type fingerprint struct{ code, resType, resName, ns string }

	olderSet := make(map[fingerprint]ErrorRecord, len(olderErrors))
	for _, e := range olderErrors {
		olderSet[fingerprint{e.ErrorCode, e.ResourceType, e.ResourceName, e.Namespace}] = e
	}
	newerSet := make(map[fingerprint]ErrorRecord, len(newerErrors))
	for _, e := range newerErrors {
		newerSet[fingerprint{e.ErrorCode, e.ResourceType, e.ResourceName, e.Namespace}] = e
	}

	diff := &ScanDiff{OlderScan: olderScan, NewerScan: newerScan}
	for fp, e := range newerSet {
		if _, exists := olderSet[fp]; !exists {
			diff.NewErrors = append(diff.NewErrors, e)
		} else {
			diff.Unchanged++
		}
	}
	for fp, e := range olderSet {
		if _, exists := newerSet[fp]; !exists {
			diff.FixedErrors = append(diff.FixedErrors, e)
		}
	}

	return diff, nil
}

// SaveDiagnostic persists a diagnostic result.
func (s *Store) SaveDiagnostic(record *DiagnosticRecord) error {
	return s.db.Create(record).Error
}

// GetDiagnosticHistory returns recent diagnostics for a resource.
func (s *Store) GetDiagnosticHistory(namespace, resourceName string, limit int) ([]DiagnosticRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	var records []DiagnosticRecord
	err := s.db.Where("namespace = ? AND resource_name = ?", namespace, resourceName).
		Order("ran_at desc").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// Close closes the database connection.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// DumpJSON writes a FaultGraph to ~/.kogaro/last-scan.json.
func DumpJSON(fg *graph.FaultGraph) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".kogaro")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "last-scan.json"), data, 0644)
}
