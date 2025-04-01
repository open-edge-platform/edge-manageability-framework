# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

variable "requester_vpc_id" {
    type = string
    description = "ID of the requester VPC"
}

variable "management_vpc_id" {
    type = string
    description = "ID of the management VPC"
}

variable "remote_vpc_dns_resolution"{
    type = bool
    default = true
}

variable "requester_vpc_routetable_ids" {
    type = list(string)
    description = "List of requester/eks vpc routetable ids"
}

variable "management_vpc_routetable_ids" {
    type = list(string)
    description = "List of management vpc routetable ids"
}
