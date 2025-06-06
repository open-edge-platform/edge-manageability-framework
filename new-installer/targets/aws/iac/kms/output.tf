# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

output "vault_kms_unseal_aws_access_key_id" {
  description = "The ID of the KMS key"
  value       = aws_iam_access_key.vault.id
  sensitive   = true
}

output "vault_kms_unseal_aws_secret_access_key" {
  description = "The secret access key for the KMS key"
  value       = aws_iam_access_key.vault.secret
  sensitive   = true
}
