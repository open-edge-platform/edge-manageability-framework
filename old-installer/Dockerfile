# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

FROM debian:bookworm-slim

ARG http_proxy
ARG https_proxy
ARG no_proxy

RUN apt update && apt install -y \
    curl \
    gettext \
    git \
    iptables \
    dnsutils \
    lsb-release \
    make \
    netcat-openbsd \
    procps \
    python3 \
    python3-pip \
    sudo \
    unzip \
    vim \
    wget \
    whiptail \
    && apt clean

# setuid \

RUN echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list
RUN wget -qO- https://www.postgresql.org/media/keys/ACCC4CF8.asc | tee /etc/apt/trusted.gpg.d/pgdg.asc
RUN apt update && apt install -y postgresql-client-15 && apt clean

RUN wget "https://github.com/oras-project/oras/releases/download/v1.1.0/oras_1.1.0_linux_amd64.tar.gz" && \
    tar zxvf oras_1.1.0_linux_amd64.tar.gz -C /usr/local/bin && \
    rm oras_1.1.0_linux_amd64.tar.gz

RUN wget https://github.com/tmccombs/hcl2json/releases/download/v0.6.4/hcl2json_linux_amd64 -O /tmp/hcl2json -q && \
    chmod +x /tmp/hcl2json && \
    mv /tmp/hcl2json /usr/local/bin/hcl2json

RUN pip install  --break-system-packages requests==2.32.0 certbot==3.2.0 \
    certbot-dns-route53==2.9.0 sshuttle==1.1.2 boto3==1.34.150

COPY asdf-install-plugins /bin/asdf-install-plugins
COPY asdf-install-versions /bin/asdf-install-versions

WORKDIR /root
USER root
COPY tool-versions .tool-versions

RUN git clone https://github.com/asdf-vm/asdf.git .asdf --branch v0.15.0
RUN echo '\nsource $HOME/.asdf/asdf.sh' >> .bashrc
RUN echo '\nexport PS1="orchestrator-admin:\W$ "' >> .bashrc

RUN bash -ic "/bin/asdf-install-plugins; /bin/asdf-install-versions"

ARG DEPLOY_TARBALL
#ARG CONFIGS_TARBALL
ARG POD_CONFIGS_TARBALL
ARG DEPLOY_TYPE
ENV DEBIAN_FRONTEND=noninteractive DEPLOY_TYPE=$DEPLOY_TYPE SSHUTTLE_ARGS="--disable-ipv6" USER=root

COPY Makefile configure-cluster.sh initialize-gitops-repos.sh start-tunnel.sh utils.sh await-argo.sh \
    prepare-upgrade.sh update-cluster.sh query-git-user.sh get-argo-login.sh cluster.tpl \
    ./
ADD $DEPLOY_TARBALL .
ADD $POD_CONFIGS_TARBALL .

# TODO: Fix issue with using ADD with the $CONFIGS_TARBALL
#COPY $CONFIGS_TARBALL .
#RUN tar -xzvf $CONFIGS_TARBALL

COPY docker-entrypoint.sh /bin/docker-entrypoint.sh
RUN chmod +x /bin/docker-entrypoint.sh
ENTRYPOINT ["/bin/docker-entrypoint.sh"]

CMD ["bash"]
