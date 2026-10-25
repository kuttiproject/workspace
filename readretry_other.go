//go:build !windows

package workspace

func istransientfileerror(err error) bool {
	return false
}
