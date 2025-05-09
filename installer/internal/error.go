package internal

type OrchInstallerErrorCode int

const (
	OrchInstallerErrorCodeUnknown OrchInstallerErrorCode = iota
	OrchInstallerErrorCodeInternal
	OrchInstallerErrorCodeInvalidArgument
	OrchInstallerErrorCodeTerraform
)

type OrchInstallerError struct {
	ErrorCode  OrchInstallerErrorCode
	ErrorMsg   string
	ErrorStage string
	ErrorStep  string
}

func (e *OrchInstallerError) Error() string {
	return e.ErrorMsg
}
