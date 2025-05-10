package internal

type OrchInstallerInputOutput interface {
	SerializeToYAML() ([]byte, error)
	DeserializeFromYAML(data []byte) error
	Validate() error
}

type OrchInstallerError struct {
	ErrorMsg string
}

func (e *OrchInstallerError) Error() string {
	return e.ErrorMsg
}
