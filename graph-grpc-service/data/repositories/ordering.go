package repositories

import "anahita-libs/data"

// safeOrderColumn is the check every sort column passes before it is put
// into a query: see anahita-libs/data, ordering.go.
var safeOrderColumn = data.SafeOrderColumn
