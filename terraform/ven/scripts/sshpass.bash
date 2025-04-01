#!/usr/bin/bash

# Define variables
password=$SSH_PASSWORD
command="$@"

# Use sshpass to handle the password prompt and execute the command
sshpass -p "$password" $command
