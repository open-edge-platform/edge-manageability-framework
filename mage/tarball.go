// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/bitfield/script"
	"github.com/magefile/mage/mg"
)

const (
	edgeManageabilityFramework = "edge-manageability-framework"
)

type TarballManifest struct {
	variant      string
	repoName     string
	actualDir    string
	clusterNames []string
	manifest     []string
}

// type clusterYaml struct {
// 	Root struct {
// 		ClusterValues []string `yaml:"clusterValues"`
// 	} `yaml:"root"`
// }

func (t Tarball) setupCollectors(variant string, clusterNames []string) error {
	tmDeploy := NewTarballManifest(variant, edgeManageabilityFramework, ".", clusterNames)
	err := tmDeploy.gatherFiles(variant)
	if err != nil {
		return err
	}
	if err := tmDeploy.writeOutTar(tmDeploy.repoName, variant); err != nil {
		return err
	}

	return nil
}

func NewTarballManifest(variant, repoName, actualName string, clusterNames []string) *TarballManifest {
	return &TarballManifest{
		variant:      variant,
		repoName:     repoName,
		actualDir:    actualName,
		clusterNames: clusterNames,
		manifest:     make([]string, 0),
	}
}

func (tm *TarballManifest) gatherFiles(variant string) error {
	switch tm.repoName {
	case edgeManageabilityFramework:
		if err := tm.addedgeManageabilityFrameworkFiles(); err != nil {
			return err
		}
		if err := tm.addCommitId("."); err != nil {
			return err
		}
	}
	return nil
}

func (tm *TarballManifest) addedgeManageabilityFrameworkFiles() error {
	tm.manifest = append(tm.manifest, tm.actualDir+"/VERSION")
	tm.manifest = append(tm.manifest, tm.actualDir+"/argocd")
	tm.manifest = append(tm.manifest, tm.actualDir+"/bootstrap")
	tm.manifest = append(tm.manifest, tm.actualDir+"/orch-configs")
	tm.manifest = append(tm.manifest, tm.actualDir+"/tools")
	return nil
}

func (tm *TarballManifest) writeOutTar(repo string, variant string) error {
	version, err := getVersionFromFile()
	if err != nil {
		return err
	}

	manifestFile, err := tm.writeManifest(fmt.Sprintf("%s.%s.manifest", variant, version))
	if err != nil {
		return err
	}
	err = tm.writeOutTarfile(variant, repo, version, manifestFile)
	if err != nil {
		return err
	}
	return nil
}

func (tm *TarballManifest) addCommitId(dir string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(dir)
	if err != nil {
		return err
	}
	count, err := script.Exec("git rev-parse --short HEAD").WriteFile("COMMIT_ID")
	if err != nil {
		return fmt.Errorf("failed writing COMMIT_ID %w", err)
	}
	if count == 0 {
		return fmt.Errorf("0 bytes written to %s/COMMIT_ID for 'git rev-parse --short HEAD' Expected more", dir)
	}
	err = os.Chdir(wd)
	if err != nil {
		return err
	}
	tm.manifest = append(tm.manifest, dir+"/COMMIT_ID")
	return nil
}

func (tm *TarballManifest) writeManifest(name string) (string, error) {
	f, err := os.CreateTemp("", name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, item := range tm.manifest {
		_, err = f.WriteString(fmt.Sprintf("%s\n", item))
		if err != nil {
			return "", err
		}
	}
	_, err = f.WriteString("\n")
	return f.Name(), err
}

func (tm *TarballManifest) writeOutTarfile(variant, repo, version string, manifestFile string) error {
	verbose := ""
	if mg.Verbose() {
		verbose = "v"
		fmt.Printf("Tar manifest file: %s\n", manifestFile)
	} else {
		defer os.Remove(manifestFile)
	}

	outdir := os.Getenv("TARBALL_DIR")
	tarFileName := path.Join(outdir, fmt.Sprintf("%s_%s_%s.tgz", variant, repo, version))

	tarCmd := fmt.Sprintf("/usr/bin/tar -cz%sf %s -T %s -P --transform s#%s#%s#",
		verbose, tarFileName, manifestFile, tm.actualDir, tm.repoName)
	if runtime.GOOS == "darwin" {
		// RegEx for replace pattern does not work properly in bsdtar, so we use alternate syntax
		tarCmd = fmt.Sprintf("/usr/bin/bsdtar -cz%sf %s -T %s -s '#%s#%s#'",
			verbose, tarFileName, manifestFile, tm.actualDir, tm.repoName)
	}
	if mg.Verbose() {
		fmt.Println(tarCmd)
	}
	if _, err := script.Exec(tarCmd).Stdout(); err != nil {
		return err
	}
	fmt.Printf("Tarball written to %s\n", tarFileName)
	return nil
}
