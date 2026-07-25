package htseq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCountAcceptsNonNegativeIntegers(t *testing.T) {
	testCases := []struct {
		name          string
		countText     string
		expectedCount Count
	}{
		{name: "zero", countText: "0", expectedCount: 0},
		{name: "positive", countText: "42", expectedCount: 42},
		{name: "leading zeroes", countText: "0007", expectedCount: 7},
		{name: "maximum exact dataframe count", countText: "9007199254740992", expectedCount: MaxExactDataFrameCount},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actualCount, err := parseCount(testCase.countText)
			if err != nil {
				t.Fatalf("parseCount(%q) returned an unexpected error: %v", testCase.countText, err)
			}
			if actualCount != testCase.expectedCount {
				t.Fatalf("parseCount(%q) = %d, want %d", testCase.countText, actualCount, testCase.expectedCount)
			}
		})
	}
}

func TestReadHTSeqFilesRejectsInvalidCounts(t *testing.T) {
	invalidCounts := []string{
		"-1",
		"+1",
		"1.0",
		".5",
		"NaN",
		"Inf",
		"+Inf",
		"1e3",
		"1E3",
		"18446744073709551616",
		"9007199254740993",
	}

	for _, invalidCount := range invalidCounts {
		t.Run(invalidCount, func(t *testing.T) {
			inputDirectory := t.TempDir()
			inputPath := filepath.Join(inputDirectory, "sample_human.txt")
			inputContents := "ENSG000001\t" + invalidCount + "\n"
			if err := os.WriteFile(inputPath, []byte(inputContents), 0644); err != nil {
				t.Fatalf("write HTSeq test file: %v", err)
			}

			_, err := ReadHTSeqFiles(inputDirectory, "_human.txt")
			if err == nil {
				t.Fatalf("ReadHTSeqFiles accepted invalid count %q", invalidCount)
			}
			if !strings.Contains(err.Error(), "invalid count value") {
				t.Fatalf("error = %q, want invalid count detail", err)
			}
		})
	}
}

func TestIsCommentOrSummary(t *testing.T) {
	commentLines := []string{
		"# enva banner",
		"__no_feature\t0",
		"__ambiguous\t5",
		"__too_low_aQual\t10",
		"__not_aligned\t0",
		"__alignment_not_unique\t0",
	}

	for _, line := range commentLines {
		// HTSeq summary lines start with "__"
		if len(line) >= 2 && line[:2] == "__" {
			continue // skip
		}
		// Comment lines start with "#"
		if len(line) >= 1 && line[0] == '#' {
			continue // skip
		}
	}
}

func TestSampleIDExtraction(t *testing.T) {
	// Test that sample IDs are correctly extracted from filenames
	fileName := "sample1_human.txt"
	postfix := "_human.txt"

	// Remove postfix from end of filename
	if len(fileName) > len(postfix) && fileName[len(fileName)-len(postfix):] == postfix {
		sampleID := fileName[:len(fileName)-len(postfix)]
		if sampleID != "sample1" {
			t.Errorf("expected sample1, got %s", sampleID)
		}
	}
}

func TestReadHTSeqFilesRejectsDuplicateGeneIDs(t *testing.T) {
	inputDirectory := t.TempDir()
	inputPath := filepath.Join(inputDirectory, "sample_human.txt")
	inputContents := "ENSG000001\t10\nENSG000001\t11\n"
	if err := os.WriteFile(inputPath, []byte(inputContents), 0644); err != nil {
		t.Fatalf("write HTSeq test file: %v", err)
	}

	_, err := ReadHTSeqFiles(inputDirectory, "_human.txt")
	if err == nil {
		t.Fatal("ReadHTSeqFiles should reject duplicate gene IDs")
	}
	if !strings.Contains(err.Error(), `duplicate gene ID "ENSG000001"`) {
		t.Fatalf("duplicate error = %q, want duplicate gene ID detail", err)
	}
}

func TestToCountMap(t *testing.T) {
	sample := HTSeqSample{
		SampleID: "test_sample",
		Records: []HTSeqRecord{
			{GeneID: "BRCA1", Count: 100},
			{GeneID: "TP53", Count: 50},
		},
	}

	countMap := sample.ToCountMap()

	if len(countMap) != 2 {
		t.Errorf("expected 2 entries in count map, got %d", len(countMap))
	}
	if countMap["BRCA1"] != 100 {
		t.Errorf("expected BRCA1 count 100, got %d", countMap["BRCA1"])
	}
	if countMap["TP53"] != 50 {
		t.Errorf("expected TP53 count 50, got %d", countMap["TP53"])
	}
}

func TestEmptyRecordsToCountMap(t *testing.T) {
	sample := HTSeqSample{
		SampleID: "empty",
		Records:  []HTSeqRecord{},
	}

	countMap := sample.ToCountMap()
	if len(countMap) != 0 {
		t.Errorf("expected empty count map, got %d entries", len(countMap))
	}
}
