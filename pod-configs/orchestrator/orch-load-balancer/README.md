# Orchestrator Load Balancers

This module defines the following:

- Application load balancer (ALB) for Traefik
- Optional second Traefik load balancer (NLB)
- Application load balancer for ArgoCD (optional)
- Target groups for:
  - Traefik (HTTPS and gRPC)
  - ArgoCD
  - Gitea
  - Nginx
- WAF WebACL attached to both ALBs for enhanced security
- Target group attachments to connect load balancers to Kubernetes services
- Security group configurations for load balancer to node communications

## Components

### Load Balancers

- **Traefik ALB**: Main application load balancer for HTTP/HTTPS traffic
- **Traefik2 NLB**: Optional network load balancer (created when `create_traefik2_load_balancer = true`)
- **ArgoCD ALB**: Optional dedicated load balancer for ArgoCD and Gitea (created when `create_argocd_load_balancer = true`)

### Security

- WAF WebACL configurations for both ALBs
- IP allowlist functionality
- Security group rules for proper communication between load balancers and EKS nodes

### Target Group Bindings

Created when `create_target_group_binding = true` for the following services:

- traefik-https
- traefik-grpc
- ingress-nginx-controller
- argocd-server
- gitea-http
