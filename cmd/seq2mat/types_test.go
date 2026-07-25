package main

import (
	"encoding/json"
	"testing"

	"github.com/rainoffallingstar/seq2mat/internal/database"
)

func TestMatrixManifestUsesSeq2matSchemas(t *testing.T) {
	manifest := MatrixManifest{
		SchemaVersion: MatrixSchemaVersion,
		Mapping: database.MappingManifest{
			SchemaVersion: database.MappingSchemaVersion,
		},
	}

	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal matrix manifest: %v", err)
	}
	var decodedManifest struct {
		SchemaVersion string `json:"schema_version"`
		Mapping       struct {
			SchemaVersion string `json:"schema_version"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(encodedManifest, &decodedManifest); err != nil {
		t.Fatalf("unmarshal matrix manifest: %v", err)
	}
	if decodedManifest.SchemaVersion != "seq2mat.matrix/1.0.0" {
		t.Fatalf("matrix schema version = %q", decodedManifest.SchemaVersion)
	}
	if decodedManifest.Mapping.SchemaVersion != "seq2mat.mapping/1.0.0" {
		t.Fatalf("mapping schema version = %q", decodedManifest.Mapping.SchemaVersion)
	}
}
