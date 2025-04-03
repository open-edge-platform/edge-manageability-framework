#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# ChangeMeOn1stLogin!

PFLOG="port-forward.log"
PF_ADDRESS=${PF_ADDRESS:-0.0.0.0}
port_forward() {
    local NS=$1; shift
    local SVC=$1; shift
    local PORTS="$*"
    local TAG=$SVC-$NS-

    (set -x; _TAG="$TAG" bash -c "while true; do kubectl port-forward --address $PF_ADDRESS -n $NS service/$SVC $PORTS; done" >>"$PFLOG" 2>&1 &) >>"$PFLOG" 2>&1
}

kill_port_forward() {
    local TAG P_IDS PARENTS KIDS UNKNOWN PP_ID NS

    NS=$1; shift
    SVC=$1; shift

    TAG=
    if [ "$SVC" != "__ALL__" ]; then
        TAG=$SVC-$NS-$NAME
    fi
    PARENTS=
    KIDS=
    UNKNOWN=
    # shellcheck disable=SC2009
    P_IDS="$(ps e -ww -A | grep "_TAG=$TAG" | grep -v grep | awk '{print $1}')"
    if [ -n "$P_IDS" ]; then
        for P_ID in $P_IDS; do
            PP_ID="$(ps -o ppid "$P_ID" | tail -n +2)"
            if [ -n "$PP_ID" ]; then
                if [ "$PP_ID" -eq 1 ]; then
                    PARENTS="$PARENTS $P_ID"
                else
                    KIDS="$KIDS $P_ID"
                fi
            else
                UNKNOWN="$UNKNOWN $P_ID"
            fi
        done
        if [ -n "$PARENTS" ]; then
            # shellcheck disable=SC2086
            while ps -h $PARENTS >/dev/null 2>&1; do
                (set -x; eval "kill -9 $PARENTS" >>"$LOG" 2>&1) >>"$LOG" 2>&1
            done
        fi
        if [ -n "$KIDS" ]; then
            # shellcheck disable=SC2086
            while ps -h $KIDS >/dev/null 2>&1; do
                (set -x; eval "kill -9 $KIDS" >>"$LOG" 2>&1) >>"$LOG" 2>&1
            done
        fi
        if [ -n "$UNKNOWN" ]; then
            # shellcheck disable=SC2086
            while ps -h $UNKNOWN >/dev/null 2>&1; do
                (set -x; eval "kill -9 $UNKNOWN" >>"$LOG" 2>&1) >>"$LOG" 2>&1
            done
        fi
    fi
}

CLUSTER_FQDN="kind.internal"

mage deploy:orchLocal dev

# mage argo:login

# disable auto-sync cause we're going to do bad, bad things
argocd app set dev/root-app --sync-policy none

# force a sync of the UI root app, we don't care about the rest of the apps right now
argocd app sync dev/web-ui-root

kubectl get configmaps -n orch-ui web-ui-root-runtime-config -o yaml | yq .data.config

# kill any old port forwards
kill_port_forward argocd argocd-server
kill_port_forward orch-platform platform-keycloak
kill_port_forward orch-ui web-ui-root

# forward everything we need
port_forward argocd argocd-server 3000:80
port_forward orch-platform platform-keycloak 4000:8080
port_forward orch-ui web-ui-root 8080:3000

# allow localhost:8080 to be a valid redirect URI for the webui-client in Keycloak
KC_USR="admin"
KC_PWD=$(kubectl -n orch-platform get secret platform-keycloak -o jsonpath='{.data.admin-password}' | base64 -d)

REALM_NAME="master"

export KC_ADMIN_TOKEN=$(curl -k -s -X POST "https://keycloak.${CLUSTER_FQDN}/realms/${REALM_NAME}/protocol/openid-connect/token" -d "username=${KC_USR}" -d "password=${KC_PWD}" -d "grant_type=password" -d "client_id=system-client" -d "scope=openid" | jq -r '.access_token')

UI_CLIENT_ID=$(curl -k -X GET \
    -H "Authorization: Bearer $KC_ADMIN_TOKEN" \
    "https://keycloak.${CLUSTER_FQDN}/admin/realms/$REALM_NAME/clients?clientId=webui-client" | jq -r ".[0].id")


curl -X PUT \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $KC_ADMIN_TOKEN" \
  -d '{
    "redirectUris": ["http://localhost/*", "http://localhost:8080/*"]
  }' \
  "https://keycloak.${CLUSTER_FQDN}/admin/realms/${REALM_NAME}/clients/${UI_CLIENT_ID}"

# rewrite the UI configmap to use localhost:4000 for KC and localhost:5000 for the API
