package database

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"sync"
)

//go:embed gene_mapping_*.csv
var csvFiles embed.FS

// EmbeddedDatabase implements GeneDatabase using embedded CSV files
// This allows the gene database to be embedded directly in the binary
type EmbeddedDatabase struct {
	humanMap map[string][]string
	mouseMap map[string][]string
	mu       sync.RWMutex
	loaded   bool
}

// NewEmbeddedDatabase creates a new embedded database instance
func NewEmbeddedDatabase() *EmbeddedDatabase {
	return &EmbeddedDatabase{
		humanMap: make(map[string][]string),
		mouseMap: make(map[string][]string),
	}
}

// LoadSpecies loads the gene database for a specific species from embedded files
func (e *EmbeddedDatabase) LoadSpecies(_, species string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// If already loaded, return success
	if species == "human" && len(e.humanMap) > 0 {
		return nil
	}
	if species == "mouse" && len(e.mouseMap) > 0 {
		return nil
	}

	// Determine which file to load
	var filename string
	var targetMap map[string][]string
	if species == "human" {
		filename = "gene_mapping_human.csv"
		targetMap = e.humanMap
	} else {
		filename = "gene_mapping_mouse.csv"
		targetMap = e.mouseMap
	}

	// Read from embedded filesystem
	file, err := csvFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open embedded file %s: %w", filename, err)
	}
	defer file.Close()

	// Parse CSV
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	// Skip header if present
	startIdx := 0
	if len(records) > 0 && (records[0][0] == "gene_id" || records[0][0] == "GeneID" || records[0][0] == "input_id") {
		startIdx = 1
	}

	for i := startIdx; i < len(records); i++ {
		record := records[i]
		if len(record) < 2 {
			continue
		}
		geneID := strings.TrimSpace(record[0])
		symbol := strings.TrimSpace(record[1])
		if geneID == "" || symbol == "" || strings.EqualFold(geneID, "NA") || strings.EqualFold(symbol, "NA") {
			continue
		}
		targetMap[geneID] = appendUniqueSymbol(targetMap[geneID], symbol)
	}

	e.loaded = true
	return nil
}

// GetSymbol returns the first symbol for compatibility.
func (e *EmbeddedDatabase) GetSymbol(geneID string) (string, bool) {
	symbols := e.GetSymbolsBySpecies(geneID, "human")
	if len(symbols) == 0 {
		symbols = e.GetSymbolsBySpecies(geneID, "mouse")
	}
	if len(symbols) == 0 {
		return "", false
	}
	return symbols[0], true
}

// GetSymbolBySpecies returns the first symbol for compatibility.
func (e *EmbeddedDatabase) GetSymbolBySpecies(geneID, species string) (string, bool) {
	symbols := e.GetSymbolsBySpecies(geneID, species)
	if len(symbols) == 0 {
		return "", false
	}
	return symbols[0], true
}

// GetSymbolsBySpecies returns all distinct symbols in source order.
func (e *EmbeddedDatabase) GetSymbolsBySpecies(geneID, species string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var symbols []string
	if species == "human" {
		symbols = e.humanMap[geneID]
	} else {
		symbols = e.mouseMap[geneID]
	}
	return append([]string(nil), symbols...)
}

// Close closes the database connection (no-op for embedded)
func (e *EmbeddedDatabase) Close() error {
	return nil
}

// LoadBothSpecies loads both human and mouse databases
func (e *EmbeddedDatabase) LoadBothSpecies(_ string) error {
	if err := e.LoadSpecies("", "human"); err != nil {
		return fmt.Errorf("failed to load human database: %w", err)
	}
	if err := e.LoadSpecies("", "mouse"); err != nil {
		return fmt.Errorf("failed to load mouse database: %w", err)
	}
	return nil
}

// CheckAvailable checks if embedded CSV files are available
func CheckAvailable() bool {
	// Try to read the directory entries from embedded FS
	entries, err := csvFiles.ReadDir(".")
	if err != nil {
		return false
	}

	hasHuman := false
	hasMouse := false
	for _, entry := range entries {
		if entry.Name() == "gene_mapping_human.csv" {
			hasHuman = true
		}
		if entry.Name() == "gene_mapping_mouse.csv" {
			hasMouse = true
		}
	}

	return hasHuman && hasMouse
}

// GetEmbeddedFileCount returns the number of embedded CSV files and their names
func GetEmbeddedFileCount() (int, []string) {
	entries, err := csvFiles.ReadDir(".")
	if err != nil {
		return 0, nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csv") {
			names = append(names, entry.Name())
		}
	}
	return len(names), names
}

// ReadEmbeddedCSV reads an embedded CSV file and returns its content as string
func ReadEmbeddedCSV(filename string) (string, error) {
	file, err := csvFiles.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open embedded file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded file: %w", err)
	}

	return string(content), nil
}
