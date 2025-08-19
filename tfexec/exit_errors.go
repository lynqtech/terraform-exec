package tfexec

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

// this file contains errors parsed from stderr

var (
	stateLockErrRegexp     = regexp.MustCompile(`Error acquiring the state lock`)
	stateLockInfoRegexp    = regexp.MustCompile(`Lock Info:\n\s*ID:\s*([^\n]+)\n\s*Path:\s*([^\n]+)\n\s*Operation:\s*([^\n]+)\n\s*Who:\s*([^\n]+)\n\s*Version:\s*([^\n]+)\n\s*Created:\s*([^\n]+)\n`)
	lockIdInvalidErrRegexp = regexp.MustCompile(`Failed to unlock state: `)
)

func (tf *Terraform) wrapExitError(ctx context.Context, err error, stderr string) error {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		// not an exit error, short circuit, nothing to wrap
		return err
	}

	ctxErr := ctx.Err()

	// nothing to parse, return early
	errString := strings.TrimSpace(stderr)
	if errString == "" {
		return &unwrapper{exitErr, ctxErr}
	}

	switch {
	case stateLockErrRegexp.MatchString(stderr):
		submatches := stateLockInfoRegexp.FindStringSubmatch(stderr)
		if len(submatches) == 7 {
			return &ErrStateLocked{
				unwrapper: unwrapper{exitErr, ctxErr},

				ID:        submatches[1],
				Path:      submatches[2],
				Operation: submatches[3],
				Who:       submatches[4],
				Version:   submatches[5],
				Created:   submatches[6],
			}
		}
	case lockIdInvalidErrRegexp.MatchString(stderr):
		return &ErrLockIdInvalid{stderr: stderr}
	}

	return fmt.Errorf("%w\n%s", &unwrapper{exitErr, ctxErr}, stderr)
}

type unwrapper struct {
	err    error
	ctxErr error
}

func (u *unwrapper) Unwrap() error {
	return u.err
}

func (u *unwrapper) Is(target error) bool {
	switch target {
	case context.DeadlineExceeded, context.Canceled:
		return u.ctxErr == context.DeadlineExceeded ||
			u.ctxErr == context.Canceled
	}
	return false
}

func (u *unwrapper) Error() string {
	return u.err.Error()
}

type ErrLockIdInvalid struct {
	unwrapper

	stderr string
}

func (e *ErrLockIdInvalid) Error() string {
	return e.stderr
}

// ErrStateLocked is returned when the state lock is already held by another process.
type ErrStateLocked struct {
	unwrapper

	ID        string
	Path      string
	Operation string
	Who       string
	Version   string
	Created   string
}

func (e *ErrStateLocked) Error() string {
	tmpl := `Lock Info:
  ID:        {{.ID}}
  Path:      {{.Path}}
  Operation: {{.Operation}}
  Who:       {{.Who}}
  Version:   {{.Version}}
  Created:   {{.Created}}
`

	t := template.Must(template.New("LockInfo").Parse(tmpl))
	var out strings.Builder
	if err := t.Execute(&out, e); err != nil {
		return "error acquiring the state lock"
	}
	return fmt.Sprintf("error acquiring the state lock: %v", out.String())
}
