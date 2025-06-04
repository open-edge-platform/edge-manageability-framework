# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

daemon on;
master_process on;
worker_processes auto;

events {
    worker_connections 1024;
}

http {
    server {
        set_by_lua_block $ecr_token {
            local token_file = io.open('/data/ecr_token', 'r')
            if token_file then
                return token_file:read()
            else
                ngx.log(ngx.ERR, "Failed to open token file: /data/ecr_token")
                return ""
            end
        }
        listen 8443 ssl;

        ssl_certificate /data/tls.crt;
        ssl_certificate_key /data/tls.key;

        ssl_protocols TLSv1.2 TLSv1.3;
        ssl_ciphers HIGH:!aNULL:!MD5;

        # increases timeouts to avoid HTTP 504
        proxy_connect_timeout  3s;
        proxy_read_timeout     300s;
        proxy_send_timeout     300s;
        send_timeout           300s;

        location /v2/ {
            set $prefix "dockercache";

            if ($arg_ns = "quay.io") {
                set $prefix "quaycache";
            }
            if ($arg_ns = "ghcr.io") {
                set $prefix "ghcrcache";
            }

            if ($arg_ns = "registry.k8s.io") {
                set $prefix "k8scache";
            }

            proxy_pass https://${backend_registry}:443;
            proxy_set_header Host ${backend_registry};
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header X-Original-URI $request_uri;
            proxy_set_header  Authorization "Basic $ecr_token";
            if ($request_uri ~* "^/v2/?$") {
                break;
            }
            rewrite ^/v2/(.*)$ /v2/$prefix/$1 break;
        }
    }
}
