# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

data "aws_vpc" "eks_vpc" {
  id = var.requester_vpc_id
}

data "aws_vpc" "management_vpc" {
  id = var.management_vpc_id
}

resource "aws_vpc_peering_connection" "main" {
  peer_vpc_id = var.management_vpc_id
  vpc_id      = var.requester_vpc_id
}


resource "aws_vpc_peering_connection_accepter" "main" {
  vpc_peering_connection_id = aws_vpc_peering_connection.main.id
  auto_accept               = true

  depends_on = [aws_vpc_peering_connection.main]
}


resource "aws_vpc_peering_connection_options" "main" {
  vpc_peering_connection_id = aws_vpc_peering_connection.main.id

  depends_on = [aws_vpc_peering_connection.main]

  accepter {
    allow_remote_vpc_dns_resolution = var.remote_vpc_dns_resolution
  }

  requester {
    allow_remote_vpc_dns_resolution = var.remote_vpc_dns_resolution
  }
}


resource "aws_route" "requester-to-accepter" {
  for_each                  = toset(var.requester_vpc_routetable_ids)
  
  destination_cidr_block    = data.aws_vpc.management_vpc.cidr_block
  route_table_id            = each.value
  vpc_peering_connection_id = aws_vpc_peering_connection.main.id
}


resource "aws_route" "accepter-to-requester" {
  for_each                  = toset(var.management_vpc_routetable_ids)

  destination_cidr_block    = data.aws_vpc.eks_vpc.cidr_block
  route_table_id            = each.value
  vpc_peering_connection_id = aws_vpc_peering_connection.main.id
}