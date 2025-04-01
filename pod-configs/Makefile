# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

package:
	@./utils/provisioning/package.sh

precheck:
	@./utils/provisioning/precheck.sh

list-modules:
	@find -name main.tf | grep -v './module' | sed 's#/main.tf##g' | sed 's#\./##g'
