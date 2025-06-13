# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

resource "aws_ecs_cluster" "pull_through_cache_proxy" {
  name = "${var.cluster_name}-ptcp"
}

data "aws_iam_policy_document" "ecs_task_role" {
  version = "2012-10-17"
  statement {
    sid     = ""
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_ecs_cluster_capacity_providers" "pull_through_cache_proxy" {
  cluster_name       = aws_ecs_cluster.pull_through_cache_proxy.name
  capacity_providers = ["FARGATE"]
}

data "aws_iam_policy_document" "ecs_task_execution_role" {
  version = "2012-10-17"
  statement {
    sid     = ""
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com", "scheduler.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task_execution_role" {
  name               = "${var.cluster_name}-ecs-execution-role"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_execution_role.json
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution_role" {
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_policy" "ecs_task_execution_secrets_policy" {
  name        = "${var.cluster_name}-ecs-execution-policy"
  description = "Policy to allow ECS task execution role"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = aws_secretsmanager_secret.pull_through_cache_proxy.arn
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchGetImage",
          "ecr:GetDownloadUrlForLayer",
          "ecr:GetImageCopyStatus",
          "ecr:BatchImportUpstreamImage"
        ]
        Resource = [
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/dockercache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/ghcrcache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/quaycache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/k8scache/*"
        ]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution_secrets_policy_attachment" {
  role       = aws_iam_role.ecs_task_execution_role.name
  policy_arn = aws_iam_policy.ecs_task_execution_secrets_policy.arn
}


resource "aws_iam_role" "ecs_task_role" {
  name               = "${var.cluster_name}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_role.json
}

resource "aws_iam_policy" "ecs_task_ecr_policy" {
  name        = "${var.cluster_name}-ecr-token"
  description = "Policy to allow ECS task role to get authorization token from ECR"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:BatchImportUpstreamImage"
        ]
        Resource = [
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/dockercache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/ghcrcache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/quaycache/*",
          "arn:aws:ecr:${var.region}:${local.account_id}:repository/k8scache/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_ecr_policy_attachment" {
  role       = aws_iam_role.ecs_task_role.name
  policy_arn = aws_iam_policy.ecs_task_ecr_policy.arn
}

resource "aws_ecs_task_definition" "pull_through_cache_proxy" {
  family                   = var.cluster_name
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn

  container_definitions = jsonencode([
    {
      essential = false
      name      = "init-container"
      image     = "${local.account_id}.dkr.ecr.${var.region}.amazonaws.com/dockercache/library/alpine:3"
      essential = false
      command   = ["sh", "-c", "echo \"$NGINX_CONFIG\" > /data/nginx.conf && echo \"$TLS_KEY\" > /data/tls.key && echo \"$TLS_CERT\" > /data/tls.crt"]
      secrets = [
        {
          name      = "NGINX_CONFIG"
          valueFrom = "${aws_secretsmanager_secret.pull_through_cache_proxy.arn}:nginx_config::"
        },
        {
          name      = "TLS_KEY"
          valueFrom = "${aws_secretsmanager_secret.pull_through_cache_proxy.arn}:tls_cert_key::"
        },
        {
          name      = "TLS_CERT"
          valueFrom = "${aws_secretsmanager_secret.pull_through_cache_proxy.arn}:tls_cert_body::"
        }
      ]
      mountPoints = [
        {
          sourceVolume  = "data"
          containerPath = "/data"
          readOnly      = false
        }
      ]
    },
    {
      name      = "entrypoint"
      image     = "${local.account_id}.dkr.ecr.${var.region}.amazonaws.com/dockercache/library/alpine:3"
      essential = false
      command   = ["sh", "-c", "echo \"$NGINX_ENTRYPOINT\" > /data/nginx-entrypoint.sh"]
      mountPoints = [
        {
          sourceVolume  = "data"
          containerPath = "/data"
          readOnly      = false
        }
      ]
      secrets = [
        {
          name      = "NGINX_ENTRYPOINT"
          valueFrom = "${aws_secretsmanager_secret.pull_through_cache_proxy.arn}:nginx_entrypoint::"
        }
      ]
      environment = [
        {
          name  = "CONFIG_VERSION"
          value = aws_secretsmanager_secret_version.pull_through_cache_proxy.version_id
        }
      ]
    },
    {
      depends_on = [
        {
          containerName = "init-container"
          condition     = "SUCCESS"
        },
        {
          containerName = "entrypoint"
          condition     = "SUCCESS"
        }
      ]
      name      = "nginx"
      image     = "${local.account_id}.dkr.ecr.${var.region}.amazonaws.com/dockercache/openresty/openresty:alpine"
      essential = true
      command   = ["/bin/sh", "/data/nginx-entrypoint.sh"]
      portMappings = [
        {
          containerPort = 8443
          hostPort      = 8443
          protocol      = "tcp"
        }
      ]
      environment = [
        {
          name  = "CONFIG_VERSION"
          value = aws_secretsmanager_secret_version.pull_through_cache_proxy.version_id
        },
        {
          name  = "HTTPS_PROXY"
          value = var.https_proxy
        },
        {
          name  = "HTTP_PROXY"
          value = var.http_proxy
        },
        {
          name  = "NO_PROXY"
          value = var.no_proxy
        }
      ]
      mountPoints = [
        {
          sourceVolume  = "data"
          containerPath = "/data"
          readOnly      = false
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.pull_through_cache_proxy.name
          awslogs-region        = var.region
          awslogs-stream-prefix = "nginx"
        }
      }
    }
  ])
  volume {
    name = "data"
  }
}

resource "aws_cloudwatch_log_group" "pull_through_cache_proxy" {
  name              = var.cluster_name
  retention_in_days = 3
}

resource "aws_security_group" "ecs_service" {
  name   = "${var.cluster_name}-ptcp-ecs"
  vpc_id = var.vpc_id
}

resource "aws_security_group_rule" "alb_to_ecs_ingress" {
  type                     = "ingress"
  from_port                = 8443
  to_port                  = 8443
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.alb.id
  security_group_id        = aws_security_group.ecs_service.id
}

data "aws_vpc" "main" {
  id = var.vpc_id
}

// To access the VPC endpoints.
resource "aws_security_group_rule" "ecs_to_internet_https" {
  type              = "egress"
  from_port         = 443
  to_port           = 443
  protocol          = "TCP"
  cidr_blocks       = [data.aws_vpc.main.cidr_block]
  security_group_id = aws_security_group.ecs_service.id
}

resource "aws_ecs_service" "pull_through_cache_proxy" {
  depends_on      = [aws_ecs_task_definition.pull_through_cache_proxy, aws_lb.pull_through_cache_proxy]
  name            = var.cluster_name
  cluster         = aws_ecs_cluster.pull_through_cache_proxy.id
  task_definition = aws_ecs_task_definition.pull_through_cache_proxy.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.pull_through_cache_proxy.arn
    container_name   = "nginx"
    container_port   = 8443
  }
}
