// Package govern implements Harbor's three-layer access control:
//   - Layer 1: Connector access (which connectors can be called)
//   - Layer 2: Resource access (which resources within a connector)
//   - Layer 3: Field visibility (which fields in the response are visible)
package govern

// Visibility represents a field's access level.
type Visibility string

const (
	Public       Visibility = "public"
	Internal     Visibility = "internal"
	Confidential Visibility = "confidential"
	Restricted   Visibility = "restricted"
)

// order maps visibility levels to numeric order for comparison.
// Lower values are more permissive.
var order = map[Visibility]int{
	Public:       0,
	Internal:     1,
	Confidential: 2,
	Restricted:   3,
}

// Allowed reports whether a field with the given visibility is visible
// under the specified maximum visibility level.
func Allowed(fieldVis, maxVis Visibility) bool {
	fo, fok := order[fieldVis]
	mo, mok := order[maxVis]
	if !fok || !mok {
		return true // unknown level → allow (safe default)
	}
	return fo <= mo
}

// FilterResult holds the outcome of a govern filter pass.
type FilterResult struct {
	RemovedFields []string
	RemovedCount  int
}

// FilterItems removes fields from each item whose visibility exceeds maxVisibility.
// fieldVisibility maps field names to their declared visibility level.
// Fields not in the map default to "public" (always visible).
//
// If maxVisibility is empty or fieldVisibility is empty, no filtering is applied
// (backward compatible).
func FilterItems(items []map[string]interface{}, fieldVisibility map[string]string, maxVisibility string) ([]map[string]interface{}, FilterResult) {
	if maxVisibility == "" || len(fieldVisibility) == 0 {
		return items, FilterResult{}
	}

	maxVis := Visibility(maxVisibility)
	if _, ok := order[maxVis]; !ok {
		return items, FilterResult{} // unknown level → no filtering
	}

	// Determine which declared fields to remove.
	removeSet := map[string]bool{}
	var removedFields []string
	for field, vis := range fieldVisibility {
		if !Allowed(Visibility(vis), maxVis) {
			removeSet[field] = true
			removedFields = append(removedFields, field)
		}
	}

	if len(removeSet) == 0 {
		return items, FilterResult{}
	}

	filtered := make([]map[string]interface{}, len(items))
	for i, item := range items {
		newItem := make(map[string]interface{}, len(item))
		for k, v := range item {
			if !removeSet[k] {
				newItem[k] = v
			}
		}
		filtered[i] = newItem
	}

	return filtered, FilterResult{
		RemovedFields: removedFields,
		RemovedCount:  len(removedFields),
	}
}
