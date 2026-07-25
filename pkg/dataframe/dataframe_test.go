package dataframe

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDataFrame(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "sample1", "sample2"})

	if df.NumCols != 3 {
		t.Errorf("expected 3 columns, got %d", df.NumCols)
	}
	if df.NumRows != 0 {
		t.Errorf("expected 0 rows, got %d", df.NumRows)
	}
	if len(df.Columns) != 3 {
		t.Errorf("expected 3 column names, got %d", len(df.Columns))
	}
}

func TestDataFrame_AddRow(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "sample1", "sample2"})

	df.AddRow("BRCA1", []float64{100.0, 200.0})
	df.AddRow("TP53", []float64{50.0, 75.0})

	if df.NumRows != 2 {
		t.Errorf("expected 2 rows, got %d", df.NumRows)
	}
	if df.RowLabels[0] != "BRCA1" {
		t.Errorf("expected BRCA1, got %s", df.RowLabels[0])
	}
	if df.RowLabels[1] != "TP53" {
		t.Errorf("expected TP53, got %s", df.RowLabels[1])
	}
	if df.Data[0][0] != 100.0 {
		t.Errorf("expected 100.0, got %f", df.Data[0][0])
	}
}

func TestDataFrame_GetRowSum(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2", "s3"})
	df.AddRow("G1", []float64{1.0, 2.0, 3.0})

	sum := df.GetRowSum(0)
	if sum != 6.0 {
		t.Errorf("expected sum 6.0, got %f", sum)
	}
}

func TestDataFrame_GetRowSum_WithNaN(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2", "s3"})
	df.AddRow("G1", []float64{1.0, math.NaN(), 3.0})

	sum := df.GetRowSum(0)
	if sum != 4.0 {
		t.Errorf("expected sum 4.0 (skipping NaN), got %f", sum)
	}
}

func TestDataFrame_GetRowSum_AllNaN(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2"})
	df.AddRow("G1", []float64{math.NaN(), math.NaN()})

	sum := df.GetRowSum(0)
	if !math.IsNaN(sum) {
		t.Errorf("expected NaN, got %f", sum)
	}
}

func TestDataFrame_GetRowSum_NegInf(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2"})
	df.AddRow("G1", []float64{1.0, math.Inf(-1)})

	sum := df.GetRowSum(0)
	if !math.IsInf(sum, -1) {
		t.Errorf("expected -Inf, got %f", sum)
	}
}

func TestDataFrame_IsRowValid(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2"})
	df.AddRow("valid", []float64{10.0, 20.0})
	df.AddRow("zero", []float64{0.0, 0.0})
	df.AddRow("neg_inf", []float64{1.0, math.Inf(-1)})
	df.AddRow("all_nan", []float64{math.NaN(), math.NaN()})

	if !df.IsRowValid(0) {
		t.Error("row with positive values should be valid")
	}
	if df.IsRowValid(1) {
		t.Error("row with all zeros should be invalid")
	}
	if df.IsRowValid(2) {
		t.Error("row with -Inf should be invalid")
	}
	if df.IsRowValid(3) {
		t.Error("row with all NaN should be invalid")
	}
}

func TestDataFrame_FilterInvalidRows(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1"})
	df.AddRow("gene1", []float64{10.0})
	df.AddRow("gene2", []float64{0.0})
	df.AddRow("gene3", []float64{5.0})

	filtered := df.FilterValidRows()

	if filtered.NumRows != 2 {
		t.Errorf("expected 2 rows after filter, got %d", filtered.NumRows)
	}
}

func TestDataFrame_Normalize(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1"})
	df.AddRow("gene1", []float64{0.0}) // log2(0+1) = log2(1) = 0
	df.AddRow("gene2", []float64{1.0}) // log2(1+1) = log2(2) = 1
	df.AddRow("gene3", []float64{3.0}) // log2(3+1) = log2(4) = 2

	normalized := df.Normalize()

	if normalized.NumRows != 3 {
		t.Fatalf("expected 3 rows, got %d", normalized.NumRows)
	}

	if normalized.Data[0][0] != 0.0 {
		t.Errorf("log2(0+1) should be 0.0, got %f", normalized.Data[0][0])
	}
	if normalized.Data[1][0] != 1.0 {
		t.Errorf("log2(1+1) should be 1.0, got %f", normalized.Data[1][0])
	}
	if normalized.Data[2][0] != 2.0 {
		t.Errorf("log2(3+1) should be 2.0, got %f", normalized.Data[2][0])
	}
}

func TestDataFrame_AggregateDuplicates(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1", "s2"})
	df.AddRow("BRCA1", []float64{10.0, 20.0})
	df.AddRow("TP53", []float64{5.0, 8.0})
	df.AddRow("BRCA1", []float64{15.0, 10.0}) // duplicate BRCA1, s1 max=15, s2 max=20

	agg := df.AggregateByGene()

	if agg.NumRows != 2 {
		t.Fatalf("expected 2 unique genes, got %d", agg.NumRows)
	}

	if agg.Data[0][0] != 15.0 {
		t.Errorf("BRCA1 s1 should be max(10,15)=15, got %f", agg.Data[0][0])
	}
	if agg.Data[0][1] != 20.0 {
		t.Errorf("BRCA1 s2 should be max(20,10)=20, got %f", agg.Data[0][1])
	}
}

func TestDataFrame_WriteTSV(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_matrix.tsv")

	df := NewDataFrame([]string{"Gene", "sample1", "sample2"})
	df.AddRow("BRCA1", []float64{100.5, 200.0})
	df.AddRow("TP53", []float64{50.0, 75.3})

	err := df.WriteTSV(outputPath)
	if err != nil {
		t.Fatalf("WriteTSV failed: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output file does not exist")
	}
}

func TestDataFrame_WriteTSVReportsFlushSyncAndCloseFailures(t *testing.T) {
	dataFrame := NewDataFrame([]string{"Gene", "sample"})
	dataFrame.AddRow("BRCA1", []float64{100})

	destination := &failingSynchronizedWriteCloser{
		writeError: errors.New("injected write failure"),
		syncError:  errors.New("injected sync failure"),
		closeError: errors.New("injected close failure"),
	}

	err := dataFrame.writeTSV(destination)
	if err == nil {
		t.Fatal("writeTSV should report injected writer failures")
	}
	for _, expectedMessage := range []string{
		"flush TSV writer",
		"injected write failure",
		"sync output file",
		"injected sync failure",
		"close output file",
		"injected close failure",
	} {
		if !strings.Contains(err.Error(), expectedMessage) {
			t.Errorf("error = %q, want %q", err, expectedMessage)
		}
	}
	if !destination.syncCalled {
		t.Error("writeTSV did not call Sync after the write failure")
	}
	if !destination.closeCalled {
		t.Error("writeTSV did not call Close after the write failure")
	}
}

func TestDataFrame_WriteTSVReportsDevFullFailure(t *testing.T) {
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full is not available")
	}

	dataFrame := NewDataFrame([]string{"Gene", "sample"})
	dataFrame.AddRow("BRCA1", []float64{100})
	if err := dataFrame.WriteTSV("/dev/full"); err == nil {
		t.Fatal("WriteTSV(/dev/full) should report a persistence failure")
	}
}

type failingSynchronizedWriteCloser struct {
	writeError  error
	syncError   error
	closeError  error
	syncCalled  bool
	closeCalled bool
}

func (destination *failingSynchronizedWriteCloser) Write(data []byte) (int, error) {
	if destination.writeError != nil {
		return 0, destination.writeError
	}
	return len(data), nil
}

func (destination *failingSynchronizedWriteCloser) Sync() error {
	destination.syncCalled = true
	return destination.syncError
}

func (destination *failingSynchronizedWriteCloser) Close() error {
	destination.closeCalled = true
	return destination.closeError
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "0"},
		{1.5, "1.5"},
		{math.NaN(), "NA"},
		{math.Inf(-1), "-Inf"},
		{math.Inf(1), "Inf"},
	}

	for _, tc := range tests {
		result := formatFloat(tc.input)
		if result != tc.expected {
			t.Errorf("formatFloat(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestDataFrame_SortRows(t *testing.T) {
	df := NewDataFrame([]string{"Gene", "s1"})
	df.AddRow("ZZZ", []float64{3.0})
	df.AddRow("AAA", []float64{1.0})
	df.AddRow("MMM", []float64{2.0})

	df.SortRows()

	if df.RowLabels[0] != "AAA" {
		t.Errorf("expected AAA first, got %s", df.RowLabels[0])
	}
	if df.RowLabels[1] != "MMM" {
		t.Errorf("expected MMM second, got %s", df.RowLabels[1])
	}
	if df.RowLabels[2] != "ZZZ" {
		t.Errorf("expected ZZZ third, got %s", df.RowLabels[2])
	}
}
