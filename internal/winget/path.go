package winget

import "strings"

// samePath compares Windows paths case-insensitively, ignoring a trailing slash.
func samePath(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, `\/`), strings.TrimRight(b, `\/`))
}

// AppendPath adds dir to a ;-separated PATH value unless it is already there.
func AppendPath(list, dir string) (string, bool) {
	for _, p := range strings.Split(list, ";") {
		if p != "" && samePath(p, dir) {
			return list, false
		}
	}
	if list != "" && !strings.HasSuffix(list, ";") {
		list += ";"
	}
	return list + dir, true
}

// RemovePath drops every entry equal to dir from a ;-separated PATH value.
func RemovePath(list, dir string) (string, bool) {
	var keep []string
	removed := false
	for _, p := range strings.Split(list, ";") {
		if p != "" && samePath(p, dir) {
			removed = true
			continue
		}
		keep = append(keep, p)
	}
	return strings.Join(keep, ";"), removed
}
