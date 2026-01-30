package apperror

import (
	"errors"
	"fmt"
)

// ExitStatus represents the exit code of the application
type ExitStatus int

const (
	// Success
	ExitSuccess ExitStatus = 0

	// General Errors
	ExitUnknownError       ExitStatus = 1
	ExitTimeout            ExitStatus = 2
	ExitResourceNotFound   ExitStatus = 3
	ExitMaxFileNotFound    ExitStatus = 4
	ExitTooSlowSpeed       ExitStatus = 5
	ExitNetworkProblem     ExitStatus = 6
	ExitUnfinishedDownload ExitStatus = 7
	ExitCannotResume       ExitStatus = 8
	ExitNotEnoughSpace     ExitStatus = 9
	ExitPieceLengthDiff    ExitStatus = 10
	ExitSameFilePresent    ExitStatus = 11
	ExitDownloadingSame    ExitStatus = 12
	ExitRenameFile         ExitStatus = 13
	ExitOpenFile           ExitStatus = 14
	ExitCreateFile         ExitStatus = 15
	ExitIOError            ExitStatus = 16
	ExitCreateDir          ExitStatus = 17
	ExitNameResFailed      ExitStatus = 18
	ExitMetalinkParse      ExitStatus = 19
	ExitFtpCommand         ExitStatus = 20
	ExitFtpNetwork         ExitStatus = 21
	ExitHttpProtocol       ExitStatus = 22
	ExitHttpRedirect       ExitStatus = 23
	ExitHttpAuth           ExitStatus = 24
	ExitFormatString       ExitStatus = 25
	ExitJsonParse          ExitStatus = 26
	ExitXmlParse           ExitStatus = 27
	ExitBadUrl             ExitStatus = 28
	ExitUnknownOption      ExitStatus = 29
	ExitOptionParse        ExitStatus = 30
	ExitTooLargeFile       ExitStatus = 31
	ExitChecksum           ExitStatus = 32
)

// Error wraps an error with an exit status
type Error struct {
	Code ExitStatus
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Error %d: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("Error %d", e.Code)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new Error with a code and message
func New(code ExitStatus, msg string) *Error {
	return &Error{
		Code: code,
		Err:  errors.New(msg),
	}
}

// Wrap creates a new Error wrapping an existing error
func Wrap(code ExitStatus, err error) *Error {
	return &Error{
		Code: code,
		Err:  err,
	}
}
