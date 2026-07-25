package output

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainoffallingstar/seq2mat/pkg/dataframe"
)

func TestWriteMatricesPublishesBothMatrices(t *testing.T) {
	outputDirectory := t.TempDir()
	countDataFrame, normalizedDataFrame := testMatrices()

	if err := WriteMatrices(countDataFrame, normalizedDataFrame, outputDirectory); err != nil {
		t.Fatalf("WriteMatrices returned an unexpected error: %v", err)
	}

	countContents := readTestFile(t, filepath.Join(outputDirectory, countMatrixFilename))
	normalizedContents := readTestFile(t, filepath.Join(outputDirectory, normalizedMatrixFilename))
	if !strings.Contains(countContents, "BRCA1\t10") {
		t.Fatalf("count matrix contents = %q, want BRCA1 count", countContents)
	}
	if !strings.Contains(normalizedContents, "BRCA1\t3.459") {
		t.Fatalf("normalized matrix contents = %q, want normalized BRCA1 value", normalizedContents)
	}
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func TestWriteMatricesWithManifestPublishesProvenanceJSON(t *testing.T) {
	outputDirectory := t.TempDir()
	countDataFrame, normalizedDataFrame := testMatrices()
	manifest := map[string]any{"schema_version": "test/1.0.0", "species": "human"}

	if err := WriteMatricesWithManifest(countDataFrame, normalizedDataFrame, outputDirectory, manifest); err != nil {
		t.Fatalf("WriteMatricesWithManifest returned an unexpected error: %v", err)
	}
	var decoded map[string]any
	contents := readTestFile(t, filepath.Join(outputDirectory, "matrix_manifest.json"))
	if err := json.Unmarshal([]byte(contents), &decoded); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if decoded["schema_version"] != "test/1.0.0" || decoded["species"] != "human" {
		t.Fatalf("manifest = %v, want schema and species", decoded)
	}
}

func TestWriteMatricesWithManifestRollsBackAllArtifactsWhenMatrixPublishFails(t *testing.T) {
	outputDirectory := t.TempDir()
	countTarget := filepath.Join(outputDirectory, countMatrixFilename)
	normalizedTarget := filepath.Join(outputDirectory, normalizedMatrixFilename)
	manifestTarget := filepath.Join(outputDirectory, "matrix_manifest.json")
	writeTestFile(t, countTarget, "old count\n")
	writeTestFile(t, normalizedTarget, "old norm\n")
	writeTestFile(t, manifestTarget, "old manifest\n")

	manifestTemporaryPath, err := writeTemporaryBytes(
		outputDirectory,
		"matrix_manifest.json",
		[]byte(`{"schema_version":"new"}`),
	)
	if err != nil {
		t.Fatalf("writeTemporaryBytes returned an unexpected error: %v", err)
	}
	countDataFrame, normalizedDataFrame := testMatrices()
	injectedRename := func(oldPath, newPath string) error {
		isCountTemporaryFile := strings.HasSuffix(oldPath, ".tmp") &&
			strings.HasPrefix(filepath.Base(oldPath), "."+countMatrixFilename+".")
		if newPath == countTarget && isCountTemporaryFile {
			return errors.New("injected count publication failure")
		}
		return os.Rename(oldPath, newPath)
	}

	err = writeMatrixSetWithPreparedPublications(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		writeDataFrame,
		injectedRename,
		[]matrixPublication{{
			targetPath:    manifestTarget,
			temporaryPath: manifestTemporaryPath,
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "injected count publication failure") {
		t.Fatalf("publication error = %v, want injected count publication failure", err)
	}

	assertFileContents(t, countTarget, "old count\n")
	assertFileContents(t, normalizedTarget, "old norm\n")
	assertFileContents(t, manifestTarget, "old manifest\n")
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func TestWriteMatricesKeepsOldOutputsWhenSecondTemporaryWriteFails(t *testing.T) {
	outputDirectory := t.TempDir()
	countTarget := filepath.Join(outputDirectory, countMatrixFilename)
	normalizedTarget := filepath.Join(outputDirectory, normalizedMatrixFilename)
	writeTestFile(t, countTarget, "old count\n")
	writeTestFile(t, normalizedTarget, "old norm\n")

	countDataFrame, normalizedDataFrame := testMatrices()
	writeCallCount := 0
	injectedWriter := func(dataFrame *dataframe.DataFrame, filePath string) error {
		writeCallCount++
		if filepath.Dir(filePath) != outputDirectory {
			t.Fatalf("temporary matrix %s was not created in output directory %s", filePath, outputDirectory)
		}
		if writeCallCount == 2 {
			if err := os.WriteFile(filePath, []byte("partial normalized matrix"), 0644); err != nil {
				t.Fatalf("write injected partial matrix: %v", err)
			}
			return errors.New("injected normalized writer failure")
		}
		return dataFrame.WriteTSV(filePath)
	}

	err := writeMatrixSet(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		injectedWriter,
		os.Rename,
	)
	if err == nil || !strings.Contains(err.Error(), "injected normalized writer failure") {
		t.Fatalf("writeMatrixSet error = %v, want injected writer failure", err)
	}

	assertFileContents(t, countTarget, "old count\n")
	assertFileContents(t, normalizedTarget, "old norm\n")
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func TestWriteMatricesRollsBackBothOutputsWhenSecondPublishFails(t *testing.T) {
	outputDirectory := t.TempDir()
	countTarget := filepath.Join(outputDirectory, countMatrixFilename)
	normalizedTarget := filepath.Join(outputDirectory, normalizedMatrixFilename)
	writeTestFile(t, countTarget, "old count\n")
	writeTestFile(t, normalizedTarget, "old norm\n")

	countDataFrame, normalizedDataFrame := testMatrices()
	injectedRename := func(oldPath, newPath string) error {
		isNormalizedTemporaryFile := strings.HasSuffix(oldPath, ".tmp") &&
			strings.HasPrefix(filepath.Base(oldPath), "."+normalizedMatrixFilename+".")
		if newPath == normalizedTarget && isNormalizedTemporaryFile {
			return errors.New("injected second publish failure")
		}
		return os.Rename(oldPath, newPath)
	}

	err := writeMatrixSet(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		writeDataFrame,
		injectedRename,
	)
	if err == nil || !strings.Contains(err.Error(), "injected second publish failure") {
		t.Fatalf("writeMatrixSet error = %v, want injected publish failure", err)
	}

	assertFileContents(t, countTarget, "old count\n")
	assertFileContents(t, normalizedTarget, "old norm\n")
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func TestWriteMatricesLeavesNoPartialOutputsWhenInitialPublishFails(t *testing.T) {
	outputDirectory := t.TempDir()
	normalizedTarget := filepath.Join(outputDirectory, normalizedMatrixFilename)
	countDataFrame, normalizedDataFrame := testMatrices()

	injectedRename := func(oldPath, newPath string) error {
		isNormalizedTemporaryFile := strings.HasSuffix(oldPath, ".tmp") &&
			strings.HasPrefix(filepath.Base(oldPath), "."+normalizedMatrixFilename+".")
		if newPath == normalizedTarget && isNormalizedTemporaryFile {
			return errors.New("injected second publish failure")
		}
		return os.Rename(oldPath, newPath)
	}

	err := writeMatrixSet(
		[]matrixSpecification{
			{dataFrame: countDataFrame, filename: countMatrixFilename},
			{dataFrame: normalizedDataFrame, filename: normalizedMatrixFilename},
		},
		outputDirectory,
		writeDataFrame,
		injectedRename,
	)
	if err == nil {
		t.Fatal("writeMatrixSet should report the injected publish failure")
	}

	for _, targetPath := range []string{
		filepath.Join(outputDirectory, countMatrixFilename),
		normalizedTarget,
	} {
		if _, statError := os.Lstat(targetPath); !errors.Is(statError, os.ErrNotExist) {
			t.Fatalf("partial output %s remains after rollback: %v", targetPath, statError)
		}
	}
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func TestWriteMatricesRejectsSymbolicLinkTarget(t *testing.T) {
	outputDirectory := t.TempDir()
	externalTarget := filepath.Join(t.TempDir(), "external-count.txt")
	writeTestFile(t, externalTarget, "external sentinel\n")

	countTarget := filepath.Join(outputDirectory, countMatrixFilename)
	if err := os.Symlink(externalTarget, countTarget); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}

	countDataFrame, normalizedDataFrame := testMatrices()
	err := WriteMatrices(countDataFrame, normalizedDataFrame, outputDirectory)
	if err == nil || !strings.Contains(err.Error(), "refusing symbolic link output target") {
		t.Fatalf("WriteMatrices error = %v, want symbolic link rejection", err)
	}
	assertFileContents(t, externalTarget, "external sentinel\n")
	if fileInformation, statError := os.Lstat(countTarget); statError != nil || fileInformation.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("count target symlink changed: info=%v error=%v", fileInformation, statError)
	}
	if _, statError := os.Lstat(filepath.Join(outputDirectory, normalizedMatrixFilename)); !errors.Is(statError, os.ErrNotExist) {
		t.Fatalf("normalized output unexpectedly exists: %v", statError)
	}
	assertNoTemporaryArtifacts(t, outputDirectory)
}

func testMatrices() (*dataframe.DataFrame, *dataframe.DataFrame) {
	countDataFrame := dataframe.NewDataFrame([]string{"Gene", "sample"})
	countDataFrame.AddRow("BRCA1", []float64{10})
	return countDataFrame, countDataFrame.Normalize()
}

func writeTestFile(t *testing.T, filePath, contents string) {
	t.Helper()
	if err := os.WriteFile(filePath, []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
}

func readTestFile(t *testing.T, filePath string) string {
	t.Helper()
	contents, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}
	return string(contents)
}

func assertFileContents(t *testing.T, filePath, expectedContents string) {
	t.Helper()
	actualContents := readTestFile(t, filePath)
	if actualContents != expectedContents {
		t.Fatalf("contents of %s = %q, want %q", filePath, actualContents, expectedContents)
	}
}

func assertNoTemporaryArtifacts(t *testing.T, outputDirectory string) {
	t.Helper()
	entries, err := os.ReadDir(outputDirectory)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") || strings.HasSuffix(entry.Name(), ".backup") {
			t.Errorf("transaction artifact remains: %s", entry.Name())
		}
	}
}
