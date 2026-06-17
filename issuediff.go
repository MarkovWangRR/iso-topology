package isotopo

// IssueDiff summarises what changed between two Validate calls.
type IssueDiff struct {
	Fixed    []Issue `json:"fixed"`    // in before, not in after
	New      []Issue `json:"new"`      // in after, not in before
	Remained []Issue `json:"remained"` // in both
}

// DiffIssues compares two issue lists (before and after an edit).
// Issues are matched by (Path, Message) pair.
func DiffIssues(before, after []Issue) IssueDiff {
	type key struct{ path, msg string }
	beforeSet := map[key][]Issue{}
	for _, iss := range before {
		k := key{iss.Path, iss.Message}
		beforeSet[k] = append(beforeSet[k], iss)
	}
	afterSet := map[key][]Issue{}
	for _, iss := range after {
		k := key{iss.Path, iss.Message}
		afterSet[k] = append(afterSet[k], iss)
	}

	var diff IssueDiff

	// Remained and Fixed: iterate before
	for k, bList := range beforeSet {
		aList := afterSet[k]
		// matched count
		matched := len(bList)
		if len(aList) < matched {
			matched = len(aList)
		}
		diff.Remained = append(diff.Remained, bList[:matched]...)
		if len(bList) > matched {
			diff.Fixed = append(diff.Fixed, bList[matched:]...)
		}
	}

	// New: in after but not before (or more occurrences)
	for k, aList := range afterSet {
		bList := beforeSet[k]
		if len(aList) > len(bList) {
			diff.New = append(diff.New, aList[len(bList):]...)
		}
	}

	return diff
}
