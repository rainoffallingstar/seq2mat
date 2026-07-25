package processor

import (
	"github.com/rainoffallingstar/seq2mat/pkg/dataframe"
)

// AggregateDuplicates aggregates duplicate gene symbols by taking max value
// This replicates R's data.table behavior:
// temp[, lapply(.SD, max, na.rm=TRUE), by=Gene]
func AggregateDuplicates(df *dataframe.DataFrame) *dataframe.DataFrame {
	return df.AggregateByGene()
}
