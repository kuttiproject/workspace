package workspace

import (
	"os/exec"
	"strings"

	"github.com/kuttiproject/kuttilog"
)

// Runwithresults runs an OS process, and returns the combined stdout and stderr output.
func Runwithresults(execpath string, paramarray ...string) (result string, err error) {
	if kuttilog.V(kuttilog.Debug) {
		kuttilog.Println(kuttilog.Debug, "------------------")
		kuttilog.Println(kuttilog.Debug, "Executing command:")
		kuttilog.Println(kuttilog.Debug, execpath, strings.Join(paramarray, " "))
		kuttilog.Println(kuttilog.Debug, "------------------")
	}

	cmd := exec.Command(execpath, paramarray...)
	output, err := cmd.CombinedOutput()

	if kuttilog.V(kuttilog.Debug) {
		kuttilog.Println(kuttilog.Debug, "Execution results:")
		kuttilog.Println(kuttilog.Debug, string(output))
		if err != nil {
			kuttilog.Printf(kuttilog.Debug, "Error: %v\n", err)
		}
		kuttilog.Println(kuttilog.Debug, "==================")
	}

	result = string(output)
	return
}
