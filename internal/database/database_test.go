package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCSVDatabasePreservesDistinctOneToManyMappingsAndSkipsNA(t *testing.T) {
	temporaryDirectory := t.TempDir()
	mappingPath := filepath.Join(temporaryDirectory, "gene_mapping_human.csv")
	if err := os.WriteFile(mappingPath, []byte("gene_id\tsymbol\nENSG1\tSYMBOL_A\nENSG1\tSYMBOL_B\nENSG1\tSYMBOL_A\nNA\tSHOULD_SKIP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mousePath := filepath.Join(temporaryDirectory, "gene_mapping_mouse.csv")
	if err := os.WriteFile(mousePath, []byte("gene_id\tsymbol\nENSMUSG1\tMOUSE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	database := NewCSVDatabase()
	if err := database.LoadSpecies(temporaryDirectory, "human"); err != nil {
		t.Fatal(err)
	}
	got := database.GetSymbolsBySpecies("ENSG1", "human")
	if len(got) != 2 || got[0] != "SYMBOL_A" || got[1] != "SYMBOL_B" {
		t.Fatalf("symbols = %v, want [SYMBOL_A SYMBOL_B]", got)
	}
	if _, found := database.GetSymbolBySpecies("NA", "human"); found {
		t.Fatal("literal NA mapping should be skipped")
	}
	manifest, err := database.MappingManifest("human")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.AssociationCount != 2 || manifest.AmbiguousIDCount != 1 || manifest.SourceSHA256 == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}
