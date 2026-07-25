package processor

import (
	"fmt"

	"github.com/rainoffallingstar/seq2mat/pkg/dataframe"
)

// FilterInvalidRows removes rows with zero, -Inf, or NA sums
// This replicates R's filter logic:
// filter(row_sum != 0 & row_sum != -Inf & !is.na(row_sum))
func FilterInvalidRows(df *dataframe.DataFrame) *dataframe.DataFrame {
	result := df.FilterValidRows()
	filtered := df.NumRows - result.NumRows
	fmt.Printf("Filtered out %d invalid rows (zero/NA/-Inf sums)\n", filtered)
	return result
}
