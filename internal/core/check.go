package core

import (
	"errors"
)

// Check loads the model in dir, warns when IgnoreEntries are not gitignored,
// collects the git diff, and formats the scope report.
// Warnings do not fail the check. A dangling assignment is a warning and is not
// treated as no assignment. Missing .arch, a failed load, a failed diff, and an
// invalid glob are errors. Check does not write local.json and does not start Watch.
func Check(dir string) (report string, warnings []string, err error) {
	if dir == "" {
		dir = "."
	}
	idx, err := Open(dir)
	if err != nil {
		return "", nil, err
	}
	if w := IgnoreWarning(dir); w != "" {
		warnings = append(warnings, w)
	}
	files, err := DiffNames(dir)
	if err != nil {
		return "", warnings, err
	}
	var id NodeID
	if idx.Assignment != nil {
		id = idx.Assignment.NodeID
	}
	results, err := CheckScope(id, files, idx)
	if err != nil {
		var dangling *DanglingAssignmentError
		if errors.As(err, &dangling) {
			warnings = append(warnings, dangling.Error())
			return "", warnings, nil
		}
		return "", warnings, err
	}
	return FormatReport(results), warnings, nil
}
