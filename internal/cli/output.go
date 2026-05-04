package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// errSilent signals that the caller has already emitted the command output
// (e.g. a JSON error envelope) and Execute should exit non-zero without
// printing anything else.
var errSilent = errors.New("silent error")

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Message string `json:"message"`
}

func writeJSON(out io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}

// handleErr converts err into a JSON error envelope on the command's stdout
// when --json is set and returns errSilent so Execute exits non-zero without
// printing anything else. In plain-text mode it returns err unchanged.
func handleErr(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	if !jsonOutput {
		return err
	}
	env := errorEnvelope{Error: errorBody{Message: err.Error()}}
	if writeErr := writeJSON(cmd.OutOrStdout(), env); writeErr != nil {
		return writeErr
	}
	return errSilent
}
