package htseq

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadHTSeqFiles reads all HTSeq files matching the pattern in the directory
func ReadHTSeqFiles(dir, postfix string) ([]HTSeqSample, error) {
	pattern := filepath.Join(dir, "*"+postfix)

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find HTSeq files with pattern %s: %w", pattern, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no HTSeq files found with pattern %s in %s", postfix, dir)
	}

	samples := make([]HTSeqSample, 0, len(files))

	for _, filePath := range files {
		sample, err := readHTSeqFile(filePath, postfix, dir)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}
		samples = append(samples, sample)
		fmt.Printf("processing %s\n", sample.SampleID)
	}

	return samples, nil
}

// readHTSeqFile reads a single HTSeq file
func readHTSeqFile(filePath, postfix, htseqDir string) (HTSeqSample, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return HTSeqSample{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Extract sample ID by removing postfix and directory path
	// First get the base filename
	baseName := filepath.Base(filePath)
	// Then remove the postfix to get sample ID
	sampleID := strings.TrimSuffix(baseName, postfix)

	scanner := bufio.NewScanner(file)
	records := make([]HTSeqRecord, 0)

	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	seenGeneIDs := make(map[string]struct{})
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip comment lines (e.g., enva banner starting with #)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Skip HTSeq summary lines
		if strings.HasPrefix(line, "__") {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return HTSeqSample{}, fmt.Errorf("line %d: invalid line format (expected 2 columns): %s", lineNumber, line)
		}

		geneID := strings.TrimSpace(parts[0])
		countText := strings.TrimSpace(parts[1])
		if geneID == "" {
			return HTSeqSample{}, fmt.Errorf("line %d: empty gene ID", lineNumber)
		}
		if _, exists := seenGeneIDs[geneID]; exists {
			return HTSeqSample{}, fmt.Errorf("line %d: duplicate gene ID %q", lineNumber, geneID)
		}
		seenGeneIDs[geneID] = struct{}{}

		count, err := parseCount(countText)
		if err != nil {
			return HTSeqSample{}, fmt.Errorf("line %d: invalid count value %q: %w", lineNumber, countText, err)
		}

		records = append(records, HTSeqRecord{
			GeneID: geneID,
			Count:  count,
		})
	}

	if err := scanner.Err(); err != nil {
		return HTSeqSample{}, fmt.Errorf("error reading file: %w", err)
	}

	return HTSeqSample{
		SampleID: sampleID,
		Records:  records,
		Path:     filePath,
	}, nil
}

func parseCount(countText string) (Count, error) {
	if countText == "" {
		return 0, fmt.Errorf("count is empty")
	}
	for characterIndex := 0; characterIndex < len(countText); characterIndex++ {
		character := countText[characterIndex]
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("count must contain decimal digits only")
		}
	}

	parsedCount, err := strconv.ParseUint(countText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("count overflows uint64: %w", err)
	}
	if parsedCount > uint64(MaxExactDataFrameCount) {
		return 0, fmt.Errorf("count %d exceeds maximum exact dataframe count %d", parsedCount, MaxExactDataFrameCount)
	}

	return Count(parsedCount), nil
}

// ToCountMap converts HTSeqSample records to a map for efficient merging
func (s *HTSeqSample) ToCountMap() GeneCountMap {
	countMap := make(GeneCountMap, len(s.Records))
	for _, record := range s.Records {
		countMap[record.GeneID] = record.Count
	}
	return countMap
}
