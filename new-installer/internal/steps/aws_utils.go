package steps

type TerraformAWSBucketBackendConfig struct {
	Region string `json:"region" yaml:"region"`
	Bucket string `json:"bucket" yaml:"bucket"`
	Key    string `json:"key" yaml:"key"`
}
