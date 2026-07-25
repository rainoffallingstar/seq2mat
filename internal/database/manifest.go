package database

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
)

const MappingSchemaVersion = "seq2mat.mapping/1.0.0"

// MappingManifest describes the exact mapping table loaded by a run.
type MappingManifest struct {
	SchemaVersion    string   `json:"schema_version"`
	Species          string   `json:"species"`
	Source           string   `json:"source"`
	SourceSHA256     string   `json:"source_sha256,omitempty"`
	InputIDCount     int      `json:"input_id_count"`
	AssociationCount int      `json:"association_count"`
	AmbiguousIDCount int      `json:"ambiguous_id_count"`
	NamespaceSamples []string `json:"namespace_samples,omitempty"`
}

// ManifestProvider exposes mapping provenance without changing GeneDatabase compatibility.
type ManifestProvider interface {
	MappingManifest(species string) (MappingManifest, error)
}

func buildMappingManifest(species, source string, mappings map[string][]string, sourceBytes []byte) MappingManifest {
	inputIDs := make([]string, 0, len(mappings))
	associationCount := 0
	ambiguousCount := 0
	for inputID, symbols := range mappings {
		inputIDs = append(inputIDs, inputID)
		associationCount += len(symbols)
		if len(symbols) > 1 {
			ambiguousCount++
		}
	}
	sort.Strings(inputIDs)
	namespaceSamples := inputIDs
	if len(namespaceSamples) > 10 {
		namespaceSamples = namespaceSamples[:10]
	}
	checksum := sha256.Sum256(sourceBytes)
	return MappingManifest{
		SchemaVersion:    MappingSchemaVersion,
		Species:          species,
		Source:           source,
		SourceSHA256:     hex.EncodeToString(checksum[:]),
		InputIDCount:     len(mappings),
		AssociationCount: associationCount,
		AmbiguousIDCount: ambiguousCount,
		NamespaceSamples: append([]string(nil), namespaceSamples...),
	}
}

func (c *CSVDatabase) MappingManifest(species string) (MappingManifest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var mappings map[string][]string
	if species == "human" {
		mappings = c.humanMap
	} else {
		mappings = c.mouseMap
	}
	path := c.sourcePaths[species]
	contents, err := os.ReadFile(path)
	if err != nil {
		return MappingManifest{}, fmt.Errorf("read mapping source %s: %w", path, err)
	}
	return buildMappingManifest(species, path, mappings, contents), nil
}

func (e *EmbeddedDatabase) MappingManifest(species string) (MappingManifest, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var mappings map[string][]string
	filename := "gene_mapping_" + species + ".csv"
	if species == "human" {
		mappings = e.humanMap
	} else {
		mappings = e.mouseMap
	}
	contents, err := csvFiles.ReadFile(filename)
	if err != nil {
		return MappingManifest{}, fmt.Errorf("read embedded mapping source %s: %w", filename, err)
	}
	return buildMappingManifest(species, "embedded:"+filename, mappings, contents), nil
}
