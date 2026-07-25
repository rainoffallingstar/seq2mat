package processor

import (
	"github.com/rainoffallingstar/seq2mat/pkg/dataframe"
)

// Normalize applies log2(x+1) transformation
// This replicates R's normalization: log2(m + 1)
func Normalize(df *dataframe.DataFrame) *dataframe.DataFrame {
	return df.Normalize()
}
