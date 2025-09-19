// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package mage

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/magefile/mage/sh"
)

// getPassword retrieves the admin password for the local postgres database.
func (Database) getPassword() (string, error) {
	encPass, err := sh.Output("kubectl", "get", "secret", "--namespace", "orch-database",
		"postgresql", "-o", "jsonpath={.data.postgres-password}")
	if err != nil {
		return "", err
	}
	pass, err := base64.StdEncoding.DecodeString(encPass)
	if err != nil {
		return "", err
	}
	return string(pass), nil
}

// psql runs an interactive shell, or the given SQL statements, against the local postgres database.
func (d Database) psql(commands ...string) error {
	pgPass, err := d.getPassword()
	if err != nil {
		return err
	}
	envMap := make(map[string]string)
	envMap["POSTGRES_PASSWORD"] = pgPass

	var args []string
	args = append(args, "run", "postgresql-db-client", "--rm", "--tty", "-i",
		"--restart=Never", "--namespace", "orch-database", "--image",
		"docker.io/bitnamilegacy/postgresql:14.5.0-debian-11-r2", "--env=PGPASSWORD=$POSTGRES_PASSWORD",
		"--command", "--", "psql", "--host", "postgresql", "-U", "postgres", "-d", "postgres", "-p", "5432")

	if len(commands) > 0 {
		commandStr := strings.Join(commands, " ")
		fmt.Println("Executing SQL >", commandStr)
		args = append(args, "-c", commandStr)
	}

	err = sh.RunWithV(envMap, "kubectl", args...)
	return err
}
