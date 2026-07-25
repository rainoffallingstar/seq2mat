package main

import (
	"github.com/rainoffallingstar/seq2mat/internal/database"
	"github.com/rainoffallingstar/seq2mat/internal/processor"
)

const MatrixSchemaVersion = "seq2mat.matrix/1.0.0"

// MatrixManifest records the exact inputs and policies used to publish a matrix set.
type MatrixManifest struct {
	SchemaVersion         string                          `json:"schema_version"`
	GeneratorVersion      string                          `json:"generator_version"`
	Species               string                          `json:"species"`
	InputDirectory        string                          `json:"input_directory"`
	Postfix               string                          `json:"postfix"`
	SampleCount           int                             `json:"sample_count"`
	RowCount              int                             `json:"row_count"`
	GeneUniverse          string                          `json:"gene_universe"`
	MissingValuePolicy    string                          `json:"missing_value_policy"`
	UnmappedIDPolicy      string                          `json:"unmapped_id_policy"`
	OneToManyPolicy       string                          `json:"one_to_many_policy"`
	DuplicateSymbolPolicy string                          `json:"duplicate_symbol_policy"`
	RowOrder              string                          `json:"row_order"`
	Mapping               database.MappingManifest        `json:"mapping"`
	Conversion            processor.GeneIDConversionStats `json:"conversion"`
}
