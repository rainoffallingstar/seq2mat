package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/rainoffallingstar/seq2mat/internal/database"
	"github.com/rainoffallingstar/seq2mat/internal/htseq"
	"github.com/rainoffallingstar/seq2mat/internal/output"
	"github.com/rainoffallingstar/seq2mat/internal/processor"
)

var (
	htseqDir  = flag.String("htseq_dir", "", "Input directory containing HTSeq files (required)")
	postfix   = flag.String("postfix", "_human.txt", "File suffix pattern (default: _human.txt)")
	outputDir = flag.String("output_dir", "", "Output directory for results (required)")
	species   = flag.String("species", "", "Species: human or mouse (auto-detected from postfix if unset)")
	showVer   = flag.Bool("version", false, "Print version")
)

var Version = "0.1.0"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: seq2mat [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Printf("seq2mat %s\n", Version)
		return
	}

	// Validate required arguments
	if *htseqDir == "" {
		log.Fatal("--htseq_dir is required")
	}
	if *outputDir == "" {
		log.Fatal("--output_dir is required")
	}

	// Run the main processing logic
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	// Determine species: explicit --species flag takes priority, otherwise detect from postfix.
	species := *species
	if species == "" {
		species = database.DetectSpecies(*postfix)
	}

	// Validate species: only "human" and "mouse" are supported.
	if species != "human" && species != "mouse" {
		return fmt.Errorf(
			"unsupported species %q: must be 'human' or 'mouse'. Specify with --species or use a postfix containing 'human' or 'mouse' (got %q)",
			species, *postfix)
	}

	// Try to use embedded database first, fall back to external files
	var geneDB database.GeneDatabase
	var usingEmbedded bool

	if database.CheckAvailable() {
		// Use embedded database
		embeddedDB := database.NewEmbeddedDatabase()
		if err := embeddedDB.LoadBothSpecies(""); err != nil {
			return fmt.Errorf("failed to load embedded gene database: %w", err)
		}
		geneDB = embeddedDB
		usingEmbedded = true
		fmt.Printf("Using embedded gene database\n")
	} else {
		// Fall back to external CSV files
		// Get the directory where the executable is located
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		exeDir := filepath.Dir(exePath)

		// Try to find the database directory
		// First try relative to executable, then try current directory
		dbPath := filepath.Join(exeDir, "data")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			dbPath = "data"
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("gene database not found. Please run: Rscript convert_rda_to_csv.R")
			}
		}

		csvDB := database.NewCSVDatabase()
		if err := csvDB.LoadBothSpecies(dbPath); err != nil {
			return fmt.Errorf("failed to load gene database: %w", err)
		}
		geneDB = csvDB
		usingEmbedded = false
		fmt.Printf("Using external gene database from: %s\n", dbPath)
	}
	defer geneDB.Close()

	// Read and merge HTSeq files
	samples, err := htseq.ReadHTSeqFiles(*htseqDir, *postfix)
	if err != nil {
		return fmt.Errorf("failed to read HTSeq files: %w", err)
	}

	df, err := processor.MergeSamples(samples)
	if err != nil {
		return fmt.Errorf("failed to merge samples: %w", err)
	}

	// Convert with an explicit contract: preserve every one-to-many association and retain unmapped IDs.
	var conversionStatistics processor.GeneIDConversionStats
	df, conversionStatistics, err = processor.ConvertGeneIDsWithOptions(
		df,
		geneDB,
		species,
		processor.ConversionOptions{OneToMany: processor.OneToManyExpand, RetainUnmapped: true},
	)
	if err != nil {
		return fmt.Errorf("failed to convert gene IDs: %w", err)
	}

	// Aggregate duplicates (multiple gene IDs mapping to the same symbol), then sort by output symbol.
	df = processor.AggregateDuplicates(df)
	df = processor.FilterInvalidRows(df)
	df.SortRows()

	normalized := processor.Normalize(df)
	mappingManifestProvider, ok := geneDB.(database.ManifestProvider)
	if !ok {
		return fmt.Errorf("selected gene database does not expose mapping provenance")
	}
	mappingManifest, err := mappingManifestProvider.MappingManifest(species)
	if err != nil {
		return fmt.Errorf("failed to build mapping manifest: %w", err)
	}
	matrixManifest := MatrixManifest{
		SchemaVersion:         MatrixSchemaVersion,
		GeneratorVersion:      Version,
		Species:               species,
		InputDirectory:        *htseqDir,
		Postfix:               *postfix,
		SampleCount:           len(samples),
		RowCount:              df.NumRows,
		GeneUniverse:          "union",
		MissingValuePolicy:    "zero",
		UnmappedIDPolicy:      "retain",
		OneToManyPolicy:       string(processor.OneToManyExpand),
		DuplicateSymbolPolicy: "column_max",
		RowOrder:              "symbol_ascending",
		Mapping:               mappingManifest,
		Conversion:            conversionStatistics,
	}

	if err := output.WriteMatricesWithManifest(df, normalized, *outputDir, matrixManifest); err != nil {
		return fmt.Errorf("failed to publish output matrices: %w", err)
	}

	fmt.Printf("\nSuccessfully processed %d samples\n", len(samples))
	fmt.Printf("Output written to %s\n", *outputDir)
	fmt.Printf("  - matrix_count.txt (raw counts)\n")
	fmt.Printf("  - matrix_norm.txt (log2 normalized)\n")
	fmt.Printf(
		"Gene ID conversion: %d converted, %d retained unmapped (%.1f%% unmapped)\n",
		conversionStatistics.ConvertedCount,
		conversionStatistics.UnmappedCount,
		conversionStatistics.UnmappedRate*100,
	)

	if usingEmbedded {
		fmt.Printf("\nNote: Using embedded gene database (built into binary)\n")
	}

	return nil
}
