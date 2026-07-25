package htseq

import "math"

// NA represents missing values (R's NA)
// Not a constant because math.NaN() returns a float64
var NA = math.NaN()

// Count is an exact, non-negative HTSeq count.
type Count uint64

// MaxExactDataFrameCount is the largest integer represented exactly by float64.
const MaxExactDataFrameCount Count = 1 << 53

// Float64 converts an exact count for the existing dataframe API.
func (count Count) Float64() float64 {
	return float64(count)
}

// HTSeqRecord represents a single gene record from HTSeq output
type HTSeqRecord struct {
	GeneID string
	Count  Count
}

// HTSeqSample represents all records from a single HTSeq file
type HTSeqSample struct {
	SampleID string
	Records  []HTSeqRecord
	Path     string
}

// GeneCountMap maps GeneID to Count for efficient lookups
type GeneCountMap map[string]Count
