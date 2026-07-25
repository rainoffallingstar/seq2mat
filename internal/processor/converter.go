package processor

import (
	"fmt"

	"github.com/gerui/htseq2matrix-go/internal/database"
	"github.com/gerui/htseq2matrix-go/pkg/dataframe"
)

const HighUnmappedRateThreshold = 0.20

// OneToManyPolicy controls how an input ID with multiple symbols is converted.
type OneToManyPolicy string

const (
	OneToManyFirst  OneToManyPolicy = "first"
	OneToManyExpand OneToManyPolicy = "expand"
	OneToManyReject OneToManyPolicy = "reject"
)

// ConversionOptions makes mapping behavior explicit at the pipeline boundary.
type ConversionOptions struct {
	OneToMany      OneToManyPolicy
	RetainUnmapped bool
}

// DefaultConversionOptions preserves the historical single-symbol API while making
// the policy available to callers that need a scientific contract.
func DefaultConversionOptions() ConversionOptions {
	return ConversionOptions{OneToMany: OneToManyFirst, RetainUnmapped: true}
}

// GeneIDConversionStats describes how many input IDs were converted or retained.
type GeneIDConversionStats struct {
	TotalCount       int
	ConvertedCount   int
	UnmappedCount    int
	UnmappedRate     float64
	HighUnmappedRate bool
}

// ConvertGeneIDs converts known gene IDs to symbols and preserves unknown IDs.
func ConvertGeneIDs(
	dataFrame *dataframe.DataFrame,
	geneDatabase database.GeneDatabase,
	species string,
) (*dataframe.DataFrame, GeneIDConversionStats, error) {
	return ConvertGeneIDsWithOptions(dataFrame, geneDatabase, species, DefaultConversionOptions())
}

// ConvertGeneIDsWithOptions applies an explicit one-to-many and unmapped-ID policy.
func ConvertGeneIDsWithOptions(
	dataFrame *dataframe.DataFrame,
	geneDatabase database.GeneDatabase,
	species string,
	options ConversionOptions,
) (*dataframe.DataFrame, GeneIDConversionStats, error) {
	if dataFrame == nil {
		return nil, GeneIDConversionStats{}, fmt.Errorf("data frame is nil")
	}
	if geneDatabase == nil {
		return nil, GeneIDConversionStats{}, fmt.Errorf("gene database is nil")
	}
	if options.OneToMany == "" {
		options.OneToMany = OneToManyFirst
	}

	result := dataframe.NewDataFrame(dataFrame.Columns)
	statistics := GeneIDConversionStats{TotalCount: dataFrame.NumRows}
	mappingProvider, supportsMultipleMappings := geneDatabase.(database.GeneMappingProvider)

	for rowIndex := 0; rowIndex < dataFrame.NumRows; rowIndex++ {
		geneID := dataFrame.RowLabels[rowIndex]
		mappedSymbols := []string(nil)
		if supportsMultipleMappings {
			mappedSymbols = mappingProvider.GetSymbolsBySpecies(geneID, species)
		}
		if len(mappedSymbols) == 0 {
			if symbol, found := geneDatabase.GetSymbolBySpecies(geneID, species); found && symbol != "" {
				mappedSymbols = []string{symbol}
			}
		}

		if len(mappedSymbols) == 0 {
			statistics.UnmappedCount++
			if options.RetainUnmapped {
				result.AddRow(geneID, dataFrame.Data[rowIndex])
			}
			continue
		}

		if len(mappedSymbols) > 1 && options.OneToMany == OneToManyReject {
			return nil, statistics, fmt.Errorf("gene ID %q maps to multiple symbols: %v", geneID, mappedSymbols)
		}
		if options.OneToMany == OneToManyFirst {
			mappedSymbols = mappedSymbols[:1]
		}

		statistics.ConvertedCount++
		for _, symbol := range mappedSymbols {
			result.AddRow(symbol, dataFrame.Data[rowIndex])
		}
	}

	if statistics.TotalCount > 0 {
		statistics.UnmappedRate = float64(statistics.UnmappedCount) / float64(statistics.TotalCount)
	}
	statistics.HighUnmappedRate = statistics.UnmappedRate > HighUnmappedRateThreshold
	return result, statistics, nil
}
