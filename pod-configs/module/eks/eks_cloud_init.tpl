MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="EKSBOUNDARY"

--EKSBOUNDARY
Content-Type: text/cloud-boothook; charset="us-ascii"

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

${user_script_pre_cloud_init}

# Install cronjob to update system
echo "50 19 * * 7 root /usr/bin/dnf update -q -y >> /var/log/automaticupdates.log" | sudo tee -a /etc/crontab
echo "0 20 * * 7 root /usr/bin/dnf upgrade -q -y >> /var/log/automaticupdates.log" | sudo tee -a /etc/crontab

# 169.254.169.254 is the AWS metadata server, see https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
MAC=$(curl -s http://169.254.169.254/latest/meta-data/mac/)
VPC_CIDR=$(curl -s http://169.254.169.254/latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-blocks | xargs | tr ' ' ',')

mkdir -p /etc/systemd/system/containerd.service.d
mkdir -p /etc/systemd/system/sandbox-image.service.d

#Configure dnf to use the proxy
cloud-init-per instance dnf_proxy_config cat << EOF >> /etc/dnf/dnf.conf
proxy=${http_proxy}
EOF

#Set the proxy for future processes, and use as an include file
cloud-init-per instance proxy_config cat << EOF >> /etc/environment
http_proxy=${http_proxy}
https_proxy=${https_proxy}
HTTP_PROXY=${http_proxy}
HTTPS_PROXY=${https_proxy}
no_proxy=$VPC_CIDR,localhost,${eks_service_cidr},127.0.0.1,169.254.169.254,.internal,s3.amazonaws.com,.s3.${aws_region}.amazonaws.com,dkr.ecr.${aws_region}.amazonaws.com,ec2.${aws_region}.amazonaws.com,.eks.amazonaws.com,.elb.${aws_region}.amazonaws.com,.dkr.ecr.${aws_region}.amazonaws.com,${no_proxy}
NO_PROXY=$no_proxy
EOF

#Configure Containerd with the proxy
cloud-init-per instance containerd_proxy_config tee <<EOF /etc/systemd/system/containerd.service.d/http-proxy.conf >/dev/null
[Service]
EnvironmentFile=/etc/environment
EOF

#Configure sandbox-image with the proxy
cloud-init-per instance sandbox-image_proxy_config tee <<EOF /etc/systemd/system/sandbox-image.service.d/http-proxy.conf >/dev/null
[Service]
EnvironmentFile=/etc/environment
EOF

#Configure the kubelet with the proxy
cloud-init-per instance kubelet_proxy_config tee <<EOF /etc/systemd/system/kubelet.service.d/proxy.conf >/dev/null
[Service]
EnvironmentFile=/etc/environment
EOF

%{ if enable_cache_registry }
  # Comment out the original config lines that will conflict with the new config.
  sudo sed -ie 's/^\s*\[plugins."io.containerd.grpc.v1.cri".registry\]/#[plugins."io.containerd.grpc.v1.cri".registry]/g' /etc/eks/containerd/containerd-config.toml
  sudo sed -ie 's/^\s*config_path/#config_path/g' /etc/eks/containerd/containerd-config.toml

  # Add new config to use the docker cache registry
  sudo cat <<EOF >> /etc/eks/containerd/containerd-config.toml
[plugins."io.containerd.grpc.v1.cri".registry]
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."*"]
      endpoint = ["${cache_registry}"]
EOF
%{ endif }


cloud-init-per instance reload_daemon systemctl daemon-reload

sudo systemctl set-environment HTTP_PROXY=${http_proxy}
sudo systemctl set-environment HTTPS_PROXY=${https_proxy}
sudo systemctl set-environment NO_PROXY=$VPC_CIDR,localhost,${eks_service_cidr},127.0.0.1,169.254.169.254,.internal,s3.amazonaws.com,.s3.${aws_region}.amazonaws.com,dkr.ecr.${aws_region}.amazonaws.com,ec2.${aws_region}.amazonaws.com,.eks.amazonaws.com,.elb.${aws_region}.amazonaws.com,.dkr.ecr.${aws_region}.amazonaws.com,${no_proxy}
sudo systemctl restart containerd.service

${user_script_post_cloud_init}

--EKSBOUNDARY
Content-Type: application/node.eks.aws

apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    name: ${cluster_name}
    apiServerEndpoint: ${eks_endpoint}
    certificateAuthority: ${eks_cluster_ca}
    cidr: "${eks_service_cidr}"
  kubelet:
    flags:
      - "--node-labels=eks.amazonaws.com/nodegroup-image=${eks_node_ami_id},eks.amazonaws.com/capacityType=ON_DEMAND,eks.amazonaws.com/nodegroup=nodegroup-${cluster_name}-1"
      - "--max-pods=${max_pods}"
%{ if eks_cluster_dns_ip != "" ~}
      - "--cluster-dns=${eks_cluster_dns_ip}"
%{ endif ~}

--EKSBOUNDARY--