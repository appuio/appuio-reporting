package sourcekey

import (
	"fmt"
	"math"
	"math/bits"
	"sort"
	"strings"
)

const elementSeparator = ":"

// SourceKey represents a source key to look up dimensions objects (currently queries and products).
// It implements the lookup logic found in https://kb.vshn.ch/appuio-cloud/references/architecture/metering-data-flow.html#_system_idea.
type SourceKey struct {
	parts []string
}

// Parse parses a source key in the format of "query:zone:tenant:namespace:class" or "query:zone:tenant:namespace".
func Parse(raw string) (SourceKey, error) {
	parts := strings.Split(raw, elementSeparator)
	if parts[len(parts)-1] == "" {
		parts = parts[0 : len(parts)-1]
	}
	if len(parts) >= 4 {
		return SourceKey{parts}, nil
	}

	return SourceKey{}, fmt.Errorf("expected key with at least 4 elements separated by `%s` got %d", elementSeparator, len(parts))
}

// Tenant returns the third element of the source key which was historically used as the tenant.
//
// Deprecated: We would like to get rid of this and read the tenant from a metric label.
func (k SourceKey) Tenant() string {
	return k.parts[2]
}

// Part returns the i-th part of the source key, or an empty string if no such part exists
func (k SourceKey) Part(i int) string {
	if i < len(k.parts) {
		return k.parts[i]
	}
	return ""
}

// String returns the string representation "query:zone:tenant:namespace:class" of the key.
func (k SourceKey) String() string {
	return strings.Join(k.parts, elementSeparator)
}

// LookupKeys generates lookup keys for a dimension object in the database.
// The logic is described here: https://kb.vshn.ch/appuio-cloud/references/architecture/metering-data-flow.html#_system_idea
func (k SourceKey) LookupKeys() []string {

	keys := make([]string, 0)
	currentKeyBase := k.parts

	for len(currentKeyBase) > 1 {
		// For the base key of a given length l, the inner l-2 elements are to be replaced with wildcards in all possible combinations.
		// To that end, generate 2^(l-2) binary numbers, sort them by specificity, and then for each number generate a key where
		// for each 1-digit, the element is replaced with a wildcard (and for a 0-digit, the element is kept as-is).
		innerLength := len(currentKeyBase) - 2
		nums := makeRange(0, int(math.Pow(2, float64(innerLength))))
		sort.Sort(sortBySpecificity(nums))
		for i := range nums {
			currentKeyElements := make([]string, 0)
			currentKeyElements = append(currentKeyElements, currentKeyBase[0])
			for digit := 0; digit < innerLength; digit++ {
				if nums[i]&uint(math.Pow(2, float64(innerLength-1-digit))) > 0 {
					currentKeyElements = append(currentKeyElements, "*")
				} else {
					currentKeyElements = append(currentKeyElements, currentKeyBase[1+digit])
				}
			}
			currentKeyElements = append(currentKeyElements, currentKeyBase[len(currentKeyBase)-1])
			keys = append(keys, strings.Join(currentKeyElements, elementSeparator))
		}
		currentKeyBase = currentKeyBase[0 : len(currentKeyBase)-1]
	}
	keys = append(keys, currentKeyBase[0])
	return keys
}

// SortBySpecificity sorts an array of uints representing binary numbers, such that numbers with fewer 1-digits come first.
// Numbers with an equal amount of 1-digits are sorted by magnitude.
type sortBySpecificity []uint

func (a sortBySpecificity) Len() int      { return len(a) }
func (a sortBySpecificity) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortBySpecificity) Less(i, j int) bool {
	onesI := bits.OnesCount(a[i])
	onesJ := bits.OnesCount(a[j])
	if onesI < onesJ {
		return true
	}
	if onesI > onesJ {
		return false
	}
	return a[i] < a[j]
}

func makeRange(min, max int) []uint {
	a := make([]uint, max-min)
	for i := range a {
		a[i] = uint(min + i)
	}
	return a
}
