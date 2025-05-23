#!/bin/bash

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -ue
set -o pipefail

. utils/lib/common.sh

# Consts:
VARIABLE_CONFIG="utils/provisioning/variables.yaml"
PROFILE_CONFIG="utils/provisioning/profiles.yaml"
CERTIFICATE_CONFIG="utils/provisioning/certificates.yaml"

# Global variables:
TFVAR_FILE=""
TFVAR_JSON=""
TMP_FILE=$(mktemp 2>/dev/null) || TMP_FILE=/tmp/config-provision-$$
AWS_ACCOUNT=""
ENV_NAME=""
export TERM=vt220   # for whiptail

is_number_value() {
    echo $1 | grep -q -P '^[0-9]+(.[0-9]+)*$'
}

is_bool_value() {
    test "$1" == true || test "$1" == false
}

is_object_value() {
    echo $1 | grep -q -P "^\s*\{.*\}\s*$"
}

is_multiline_value() {
    local ln=$(echo "$1" | wc -l)
    test $ln -gt 1
}

is_string_variable() {
    var="$1"
    local content=$(yq ".[] | select(.variable == \"${var}\")" $VARIABLE_CONFIG)
    if [[ -n "$content" ]]; then
        local type=$(echo "$content" | yq ".type")
        if [[ -n "$type" ]] && [[ "$type" == "string" ]]; then
            return 0
        else
            return 1
        fi
    else
        return 1
    fi
}

is_eot_value() {
    echo $1 |grep -q -P "^\s*<<EOT"
}

is_enclosed_dq() {
    echo $1 | grep -q -P "^\".*\""
}

add_eot() {
    echo "<<EOT"$'\n'"$1"$'\n'"EOT"
}

remove_eot() {
    echo "$1" | sed -e 's|^\s*<<EOT||g' -e 's|EOT$||g'
}

escape_dq() {
    echo "$1" | sed -e 's|\\n|\\\\n|g' -e 's|\"|\\\"|g'
}

unescape_dq() {
    echo "$1" | sed -e 's|\\\"|\"|g' -e 's|\\\\n|\\n|g'
}

all_profile_instance_type() {
    # Get the instance types from profiles.yaml
    (yq '.[].variables[] | select(.variable=="eks_additional_node_groups") | .value' utils/provisioning/profiles.yaml | grep instance_type |sed -ne 's|^\s*instance_type\s*=\s*\(.\+\)\s*$|\1|p' | xargs -I {}  echo {}; \
    yq '.[].variables[] | select(.variable=="eks_node_instance_type") | .value ' utils/provisioning/profiles.yaml) | \
    sort | uniq
}

tfvar2json() {
    hcl2json $TFVAR_FILE > $TFVAR_JSON 2>/dev/null || echo "{}" > $TFVAR_JSON
}

json2tfvar() {
    local var
    rm -f $TFVAR_FILE || true
    local ks=$(jq -r 'keys_unsorted' $TFVAR_JSON)
    local i=0
    while :; do
        var=$(echo $ks | jq -r ".[$i]")
        if [[ -z $var ]] || [[ $var == null ]]; then
            break
        fi
        local val=$(jq -r ".${var}" $TFVAR_JSON)
        if is_string_variable "$var" && ! is_eot_value "$val" && ! is_enclosed_dq "$val"; then
            # if it is a string and has not been enclosed with double quotes, enclose it."
            val="\"$(escape_dq "$val")\""
        fi
        if ! is_object_value "$val" && is_multiline_value "$val"; then
            val=$(add_eot "$val")
        fi
        echo ${var}="${val}" >> $TFVAR_FILE
        (( i++ )) || true
    done
}

get_saved_value() {
    local key=$1

    if [[ ! -f $TFVAR_FILE ]]; then
        echo ""     
    else
        jq -r ".${key}" $TFVAR_JSON
    fi
}

select_list() {
    local path="$1"
    local val="$2"
    local list=()
    local item_number=0
    local items=""
    local v
    while v=$(yq -r "${path}.list[$item_number]" $VARIABLE_CONFIG) && [[ -n "$v" ]]; do
        if [[ -z "$v" ]] || [[ "$v" == "null" ]]; then
            break
        fi
        list[$item_number]=$v
        (( item_number++ )) || true
        
        local o="off"
        if [[ $v == $val ]]; then
            o="on"
        fi
        items="${items} $item_number \"${v}\" $o"
    done

    # Allow inputting empty value to remove the variable
    remove_item=$(( item_number + 1 ))
    items="${items} $remove_item \"[Select this item to reset the variable]\" off"

    cmd="whiptail --title \"$title\" --radiolist \"Select a value\" 0 0 $item_number $items 3>&1 1>&2 2>&3"
    r=$(eval $cmd) || cancel=true
    if ! $cancel; then
        if [[ $r -eq $remove_item ]]; then
            echo ""
        else
            echo ${list[$((r-1))]}
        fi
        return 0
    else
        return 1
    fi
}

select_instance_type() {
    local val="$1"
    local list=()
    local item_number=0
    local items=""
    local v
    for v in $(all_profile_instance_type); do
        list[$item_number]=$v
        (( item_number++ )) || true
        
        local o="off"
        if [[ $v == $val ]]; then
            o="on"
        fi
        items="${items} $item_number \"${v}\" $o"
    done

    # Allow inputting empty value to remove the variable
    remove_item=$(( item_number + 1 ))
    items="${items} $remove_item \"[Select this item to reset the variable]\" off"

    cmd="whiptail --title \"$title\" --radiolist \"Select a value\" 0 0 $item_number $items 3>&1 1>&2 2>&3"
    r=$(eval $cmd) || cancel=true
    if ! $cancel; then
        if [[ $r -eq $remove_item ]]; then
            echo ""
        else
            echo ${list[$((r-1))]}
        fi
        return 0
    else
        return 1
    fi
}

set_adv_variable() {
    local path="$1"
    local var_kind="$2"
    local title=$(yq "${path}.name" $VARIABLE_CONFIG)
    local var
    local field
    local var_json

    if [[ $var_kind == "variable" ]]; then
        var=$(yq -r "${path}.variable" $VARIABLE_CONFIG)
        var_json="$var"
    elif [[ $var_kind == "variableField" ]]; then
        var=$(yq -r "${path}.${var_kind}.variable" $VARIABLE_CONFIG)
        field=$(yq -r "${path}.${var_kind}.field" $VARIABLE_CONFIG)
        var_json="${var}${field}"    # No dot between
    fi

    local orig_val=$(get_saved_value $var_json)
    if [[ $var_kind == "variableField" ]] && ([[ -z "$orig_val" ]] || [[ "$orig_val" == "null" ]]); then
        # Cannot set only a field of an object if the whole object has not been set in the variable file.
        whiptail --title "Command not allowed" --msgbox "The item ${title} is a part of the object ${var}, but the object has not been set. It is not allowed to only set one field of an object. You may want to select a capacity profile and try again." 16 78
        return
    fi

    local new_value=""
    local cancel=false

    if [[ -z "$orig_val" ]] || [[ "$orig_val" == "null" ]]; then
        local val=$(yq -r "${path}.default" $VARIABLE_CONFIG) || true
    else
        local val=$orig_val
    fi

    if $(yq "$path | has(\"list\")" $VARIABLE_CONFIG); then
        # If not cancel, assign new_val with the selected value from the list:
        local ret
        if ret=$(select_list $path $val); then
            new_val="$ret"
        else
            cancel=true
        fi
    elif $(yq "$path | has(\"listFrom\")" $VARIABLE_CONFIG); then
        list_from=$(yq "${path}.listFrom" $VARIABLE_CONFIG)
        if [[ "$list_from" == "instanceTypes" ]]; then
            # If not cancel, assign new_val with the selected value from the list:
            local ret
            if ret=$(select_instance_type $val); then
                new_val="$ret"
            else
                cancel=true
            fi
        else
            # Something may be not correct. But still continue.
            cancel=false
        fi
    else
        new_val=$(whiptail --inputbox "$title" 10 30 "${val:-}" 3>&1 1>&2 2>&3) || cancel=true
    fi

    if $cancel; then
        return
    fi

    if [[ -n "$new_val" ]]; then
        whiptail --title "Info" --infobox "Saving the value..." 8 78
        set_json_variable "${var_json}" "${new_val}"
    else
        if [[ $var_kind == "variableField" ]]; then
            whiptail --title "Error input" --msgbox "Blank value is not allowed because the item ${title} is a part of the object ${var}. You may want to select an appropriate capacity profile ." 16 78
            return            
        fi
        whiptail --title "Info" --infobox "Removing the variable because the input is blank..." 8 78
        remove_json_variable "${var_json}"
    fi
    json2tfvar
}

set_json_variable() {
    local var=$1
    local new_val="$2"
    
    jq --arg nv "$new_val" '.'$var'=$nv'  $TFVAR_JSON > $TMP_FILE

    cp $TMP_FILE $TFVAR_JSON
}

remove_json_variable() {
    local var=$1

    jq "del(.${var})" $TFVAR_JSON > $TMP_FILE

    cp $TMP_FILE $TFVAR_JSON
}

config_certificate() {
    local title="Configure TLS certificate"
    local tmpfile=$(mktemp 2>/dev/null)

    # Get the certificate related variable names
    local vs=$(yq ".tls.body[]" $CERTIFICATE_CONFIG)
    local vars_body=($vs)
    vs=$(yq ".tls.key[]" $CERTIFICATE_CONFIG)
    local vars_privkey=($vs)
    vs=$(yq ".tls.chain[]" $CERTIFICATE_CONFIG)
    local vars_chain=($vs)
    vs=$(yq ".tls.bundle[]" $CERTIFICATE_CONFIG)
    local vars_bundle=($vs)

    # Get saved certificate and key contents
    local orig_body_val=$(get_saved_value ${vars_body[0]})
    orig_body_val=$(remove_eot "$orig_body_val")
    local orig_privkey_val=$(get_saved_value ${vars_privkey[0]})
    orig_privkey_val=$(remove_eot "$orig_privkey_val")
    local orig_chain_val=$(get_saved_value ${vars_chain[0]})
    orig_chain_val=$(remove_eot "$orig_chain_val")
    local orig_bundle_val=$(get_saved_value ${vars_bundle[0]})
    orig_bundle_val=$(remove_eot "$orig_bundle_val")

    echo "## Please insert or replace the content of the certificates and private key under the designated places and save the file." >$tmpfile
    echo "## Note not to remove or change the commented lines." >> $tmpfile
    echo -e "\n"
    echo "# Content of the certificate body below. If it includes the CA certificates, they will be removed." >> $tmpfile
    if [[ -n "$orig_body_val" ]] && [[ "$orig_body_val" != "null" ]]; then
        echo "$orig_body_val" >>$tmpfile
    fi
    echo -e "\n" >>$tmpfile
    echo "# Content of the private key below:" >> $tmpfile
    if [[ -n "$orig_privkey_val" ]] && [[ "$orig_privkey_val" != "null" ]]; then
        echo "$orig_privkey_val" >>$tmpfile
    fi
    echo -e "\n" >>$tmpfile
    echo "# Content of the CA certificate chain below: (Optional, if the content of the certificate doesn't include the CA certificate chain.)" >> $tmpfile
    if [[ -n "$orig_chain_val" ]] && [[ "$orig_chain_val" != "null" ]]; then
        echo "$orig_chain_val" >>$tmpfile
    fi
    echo -e "\n" >>$tmpfile
    cp $tmpfile ${tmpfile}.bak

    while :; do
        if ! editor $tmpfile; then
            # User does not save the file
            rm $tmpfile ${tmpfile}.bak || true
            return
        fi

        if diff $tmpfile ${tmpfile}.bak &>/dev/null; then
            whiptail --title "Values" --msgbox "There is no change in the certificate. Nothing has been saved." 0 0
            rm $tmpfile ${tmpfile}.bak || true
            return
        fi

        # Extract the first certificate which is the leaf certificate
        local body=$(grep -v -P '^\s*#' $tmpfile | grep -v -P '^\s*$' | awk -v n=1 'end==1 {n++;end=0} /-----END CERTIFICATE---/ {end++} {if(n==1) print}')
        # Extract the private key which must be the only one in the file
        local privkey=$(grep -v -P '^\s*#' $tmpfile | grep -v -P '^\s*$' | awk -v n=0 'end==1 {n++;end=0} /-----BEGIN PRIVATE KEY/ {n=1} /-----END PRIVATE KEY---/ {end++} {if(n==1) print}')
        # Extract the CA certificates after the private key if there is any
        local chain=$(grep -v -P '^\s*#' $tmpfile | grep -v -P '^\s*$' | awk -v n=0 -v start=0 '/-----END PRIVATE KEY/ {start++} /-----BEGIN CERTIFICATE/ {if(start>0) n++} {if(n>0) print}')
        if [[ -z "$chain" ]]; then
            # Extract the CA certificates before the private key if there is any
            chain=$(grep -v -P '^\s*#' $tmpfile | awk -v n=0 -v end=0 '/BEGIN CERTIFICATE/ {n++} /-----BEGIN PRIVATE KEY/ {end=1} {if(n>1 && end==0) print}')
        fi

        if [[ -z "$body" ]] && [[ -z "$privkey" ]]; then
            whiptail --title "Info" --infobox "Removing the certificate and private key because the input for both are blank..." 8 78
            
            local v
            for v in ${vars_body[@]}; do
                remove_json_variable $v
            done
            for v in ${vars_privkey[@]}; do
                remove_json_variable $v
            done
            for v in ${vars_chain[@]}; do
                remove_json_variable $v
            done
            for v in ${vars_bundle[@]}; do
                remove_json_variable $v
            done
            json2tfvar
            rm $tmpfile ${tmpfile}.bak || true
            return
        fi

        local err_items=""
        [[ -z "$body" ]] && err_items="${err_items}certificate, "
        [[ -z "$privkey" ]] && err_items="${err_items}private key, " 
        [[ -z "$chain" ]] && err_items="${err_items}chain"
        
        if [[ -n "$err_items" ]]; then
            if whiptail --title "Error Input" --yesno --yes-button "Check" --no-button "Cancel" "The contents of ${err_items%, } are missing. Do you want to check or cancel it?" 0 0; then
                continue
            else
                rm $tmpfile ${tmpfile}.bak || true
                return
            fi
        fi

        local bundle="${body}\n${chain}"

        local ret=0
        check_cert_input "$body" "$privkey" "$chain" || ret=$?
        if [[ $ret -eq 2 ]] ; then
            # Cancel without saving the values
            rm $tmpfile ${tmpfile}.bak || true
            return
        elif [[ $ret -eq 1 ]] ; then
            # There is error. Return the menu to check.
            continue
        fi

        # No error.
        break
    done

    whiptail --title "Info" --infobox "Saving the certificates. It could take several seconds..." 8 78

    local v
    for v in ${vars_body[@]}; do
        set_json_variable $v "$body"
    done
    for v in ${vars_privkey[@]}; do
        set_json_variable $v "$privkey"
    done
    for v in ${vars_chain[@]}; do
        set_json_variable $v "$chain"
    done
    for v in ${vars_bundle[@]}; do
        set_json_variable $v "$bundle"
    done
    json2tfvar
    rm $tmpfile ${tmpfile}.bak || true
}

check_cert_input() {
    local body="$1"
    local privkey="$2"
    local chain="$3"
    local ret=0
    check_cert "$body" "certificate body" || ret=$?
    [[ $ret -gt 0 ]] && return $ret
    check_cert "$chain" "CA certificate chain" || ret=$?
    [[ $ret -gt 0 ]] && return $ret
    check_cert_pair "$body" "$privkey" || ret=$?
    [[ $ret -gt 0 ]] && return $ret

    return 0
}

check_cert() {
    local cert="$1"
    local name="$2"
    local ret=0
    local diff
    if ! diff=$(get_cert_expire "$cert"); then
        confirm_check "Error Input" "The content of $name is invalid. Do you want to check or cancel it?" || ret=$?
    else
        if [[ $diff -le 0 ]]; then
            confirm_check "Error Input" "The content of $name has expired. Do you want to check or cancel it?" || ret=$?
        fi
    fi
    return $ret
}

confirm_check() {
    title="$1"
    msg="$2"

    if whiptail --title "$title" --yesno --yes-button "Check" --no-button "Cancel" "$msg" 0 0; then
        ret=1
    else
        ret=2
    fi
    return $ret
}

check_cert_pair() {
    local cert="$1"
    local privkey="$2"

    if ! pub1=$(openssl pkey -in <(echo "$privkey") -pubout -outform pem 2>/dev/null); then
        confirm_check "Error Input" "The content of the private key is invalid. Do you want to check or cancel it?" || ret=$?
        return $ret
    fi
    
    if ! pub2=$(openssl x509 -in <(echo "$cert") -pubkey -noout -outform pem 2>/dev/null); then
        confirm_check "Error Input" "The content of the certificate is invalid. Do you want to check or cancel it?" || ret=$?
        return $ret
    fi

    if [[ "$pub1" == "$pub2" ]]; then
        return 0
    else
        # return the return value of confirm_check
        confirm_check "Error Input" "The public key of the certificate doesn't match the private key. Do you want to check or cancel it?" || ret=$?
        return $ret
    fi
}

check_prerequisites() {
    for i in whiptail hcl2json; do
        if ! command -v $i 2>&1 >/dev/null; then
            echo "Error: $i could not be found. Please install it."
            exit 1
        fi
    done

    [[ -d $SAVE_DIR ]] || mkdir -p $SAVE_DIR
}

input_value_noblank() {
    local title="$1"
    local default="$2"
    while :; do
        val=$(whiptail --inputbox "$title" 10 30 "$default" 3>&1 1>&2 2>&3) || exit
        if [[ -n $val ]]; then
            break
        fi
    done
    echo $val
}

set_conf_value() {
    if ! AWS_ACCOUNT=$(input_value_noblank "AWS Account Number:" "") || ! ENV_NAME=$(input_value_noblank "Environment Name:" ""); then
        clear
        exit
    fi
    
    TFVAR_FILE="${SAVE_DIR}/${AWS_ACCOUNT}-${ENV_NAME}-values.tfvar"
    TFVAR_JSON="${SAVE_DIR}/${AWS_ACCOUNT}-${ENV_NAME}-values.tfvar.json"
}

config_variables() {
    local items_output=$(yq '.[].name' $VARIABLE_CONFIG)
    local title="Configure Cluster Variables"
    local items=""
    local item_number=1
    local curr_select
    while read -r item; do
        items="${items} $item_number \"$item\""
        ((item_number++)) || true
    done < <(echo "$items_output")

    cmd="whiptail --clear --backtitle \"Advanced Setting Menu\" \
        --title \"${title}\" \
        --menu \"Choose an option\" 0 0 ${item_number} $items \
        --ok-button \"Select\" --cancel-button \"Return\" 3>&1 1>&2 2>&3"

    if ! curr_select=$(eval $cmd); then
        return -1
    else
        local idx=$(( curr_select - 1 ))
        if $(yq ".[$idx] | has(\"variable\")" $VARIABLE_CONFIG); then
            set_adv_variable ".[$idx]" "variable"
        elif $(yq ".[$idx]  | has(\"variableField\")" $VARIABLE_CONFIG); then
            set_adv_variable ".[$idx]" "variableField"
        fi
    fi
}

adv_menu() {
    while : ; do 
        local items='1 "Configure Cluster Variables" 2 "Configure Certificate" 3 "Show Configured Values"'
        local curr_select

        cmd="whiptail --clear --backtitle \"Advanced Setting Menu\" \
            --title \"${title}\" \
            --menu \"Choose an option\" 0 0 ${item_number} $items \
            --ok-button \"Select\" --cancel-button \"Return\" 3>&1 1>&2 2>&3"
        if ! curr_select=$(eval $cmd); then
            return
        fi

        case $curr_select in
            1) config_variables || break;;   # Exit the loop only when Cancel is selected
            2) config_certificate || break;;   # Exit the loop only when Cancel is selected;;
            3) show_values;;
            *) echo "Error: Internal error." && exit 1;;
        esac
    done
}

show_values() {
    whiptail --title "Info" --infobox "Getting all configured variables. It could take several seconds..." 8 78
    local vars_set="$(print_values)"
    local mesg="The following values have been configured:\n\n${vars_set}"
    whiptail --title "Values" --msgbox "$mesg" 0 78
}

print_values() {
    local i=0
    while :; do
        local p=".[$i]"
        local n=$(yq "${p}.name" $VARIABLE_CONFIG 2>/dev/null)      
        if [[ "$n" == "null" ]]; then       
            break
        fi

        if $(yq "$p | has(\"variable\")" $VARIABLE_CONFIG); then
            var=$(yq "${p}.variable" $VARIABLE_CONFIG 2>/dev/null)
            val=$(jq -r ".${var}" $TFVAR_JSON)
        elif $(yq "$p | has(\"variableField\")" $VARIABLE_CONFIG); then
            var=$(yq -r "${p}.variableField.variable" $VARIABLE_CONFIG)
            field=$(yq -r "${p}.variableField.field" $VARIABLE_CONFIG)
            var_json="${var}${field}"    # No dot between
            val=$(jq -r ".${var_json}" $TFVAR_JSON)
        fi

        if [[ "$val" != "null" ]]; then
            if is_multiline_value "$val" && ! is_object_value "$val" && ! is_string_variable "$var"; then
                # If it is certificate
                val=$(add_eot "$val")
            fi
            if is_string_variable "$var" &&
                ! is_eot_value "$val" && \
                ! is_enclosed_dq "$val"; then
                # if it is a string and has not been enclosed with double quotes, enclose it."
                val="\"$(escape_dq "$val")\""
            fi  
            echo -e "\t${n} ==> ${val}"
        fi
        (( i++ )) || true
    done
}

show_profile_menu() {
    local title="Select Capacity Profile"
    local items_output=$(yq -r ".[].name" $PROFILE_CONFIG)
    local items=""
    local item_number=1
    local curr_select
    while read -r item; do
        description=$(yq -r '.[] | select(.name=="'${item}'") | .description' $PROFILE_CONFIG)
        local n
        if [[ "$description" == "null" ]]; then
            n="$item"
        else
            n="$description"
        fi
        items="$items $item_number \"${n}\""
        ((item_number++)) || true
    done < <(echo "$items_output")
    local prof_num=$((item_number - 1))
    local adv_menu_index=$item_number
    (( item_number += 1 )) || true

    items="$items $adv_menu_index \"Advanced Setting ...\""

    cmd="whiptail --clear --backtitle \"Profile Menu\" \
        --title \"${title}\" \
        --menu \"Choose an option\" 0 0 ${adv_menu_index} $items \
        --ok-button \"Select\" --cancel-button \"Exit\" 3>&1 1>&2 2>&3"

    if curr_select=$(eval $cmd); then
        local index=$((curr_select - 1))

        if [[ $curr_select -eq $adv_menu_index ]]; then
            adv_menu
        else
            local profile_name=$(yq -r ".[$index].name" $PROFILE_CONFIG)
            local item_output=$(yq -r ".[$index].variables.[].name" $PROFILE_CONFIG)
            IFS=$'\n' var_names=($item_output)
            item_output=$(yq -r ".[$index].variables.[].variable" $PROFILE_CONFIG)
            IFS=$'\n' var_vars=($item_output)

            local mesg="The following values will be set for the $profile_name profile:\n\n"
            for i in $(seq 0 $((${#var_vars[@]} - 1))); do
                local v=$(yq -r ".[$index].variables.[$i].value" $PROFILE_CONFIG)
                v=$(echo $v)   # Transform to a single line
                mesg="${mesg}      ${var_names[$i]} => ${v}\n" 
            done

            if whiptail --title "Values to be set" --yesno $mesg 0 0; then
                whiptail --title "Info" --infobox "Preparing to save the profile. It could take several seconds..." 8 78
                for i in $(seq 0 $((${#var_vars[@]} - 1))); do
                    local v=$(yq -r ".[$index].variables.[$i].value" $PROFILE_CONFIG)
                    local t=$(yq -r ".[$index].variables.[$i].type" $PROFILE_CONFIG)
                    if [[ $t == string ]]; then
                        v="\"$v\""
                    fi
                    set_json_variable "${var_vars[$i]}" "${v}"
                done
                json2tfvar
                tfvar2json   # Reload the json to refresh some value formats
            fi
        fi
    else
        return 1   # exit menu
    fi
}

# main:
check_prerequisites
set_conf_value

tfvar2json
while : ; do
    show_profile_menu || break
done
clear
# rm $TFVAR_JSON || true
