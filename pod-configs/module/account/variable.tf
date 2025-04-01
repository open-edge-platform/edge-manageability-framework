# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "feature_flags" {
  type = object({
    iam_roles  = bool
  })
  default = {
    iam_roles  = true
  }
  description = <<EOT
To enable certain features:
- iam_roles: Will create IAM roles for things like clusters, EFS, ...
  EOT
}
