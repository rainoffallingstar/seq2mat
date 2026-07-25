package processor

import (
	"testing"

	"github.com/rainoffallingstar/seq2mat/internal/htseq"
)

func TestMergeSamplesFillsMissingGenesWithZero(t *testing.T) {
	samples := []htseq.HTSeqSample{
		{
			SampleID: "sample_one",
			Records: []htseq.HTSeqRecord{
				{GeneID: "ENSG000001", Count: 10},
			},
		},
		{
			SampleID: "sample_two",
			Records: []htseq.HTSeqRecord{
				{GeneID: "ENSG000002", Count: 20},
			},
		},
	}

	dataFrame, err := MergeSamples(samples)
	if err != nil {
		t.Fatalf("MergeSamples returned an unexpected error: %v", err)
	}
	if dataFrame.NumRows != 2 {
		t.Fatalf("merged row count = %d, want 2", dataFrame.NumRows)
	}
	if dataFrame.RowLabels[0] != "ENSG000001" || dataFrame.Data[0][0] != 10 || dataFrame.Data[0][1] != 0 {
		t.Errorf("first row = %q %v, want ENSG000001 [10 0]", dataFrame.RowLabels[0], dataFrame.Data[0])
	}
	if dataFrame.RowLabels[1] != "ENSG000002" || dataFrame.Data[1][0] != 0 || dataFrame.Data[1][1] != 20 {
		t.Errorf("second row = %q %v, want ENSG000002 [0 20]", dataFrame.RowLabels[1], dataFrame.Data[1])
	}
}

func TestMergeSamplesRejectsDuplicateGeneIDs(t *testing.T) {
	samples := []htseq.HTSeqSample{
		{
			SampleID: "duplicate_sample",
			Records: []htseq.HTSeqRecord{
				{GeneID: "ENSG000001", Count: 10},
				{GeneID: "ENSG000001", Count: 11},
			},
		},
	}

	if _, err := MergeSamples(samples); err == nil {
		t.Fatal("MergeSamples should reject duplicate gene IDs in one sample")
	}
}
