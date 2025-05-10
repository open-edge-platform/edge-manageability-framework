package main

import (
	. "github.com/open-edge-platform/edge-manageability-framework/installer/internal"
)

func main() {
	// Create an instance of OrchInstaller
	orchInstaller := CreateOrchInstaller()

	// Add stages to the OrchInstaller
	input := &OrchInstallerDummyStage1Input{Name: "dummy"}
	orchInstaller.Stages = CreateDummyStages(input)

	// Run the OrchInstaller
	if err := orchInstaller.Run(); err != nil {
		panic(err)
	}
}
