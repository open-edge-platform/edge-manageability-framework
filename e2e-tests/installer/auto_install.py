# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

"""
auto_install.py

This module contains a script to automate workflows for the Edge Orchestrator cloud installer
within Jenkins E2E validation pipelines.
"""

import sys
import os
import shutil
import time
import argparse
import pexpect


def last_non_whitespace_line(lines):
    """
    Scans through an array of lines to find the last one that contains non-whitespace content. If the first
    non-whitespace line starts with'****' or no non-whitespace line is found, it will return None.

    :param lines: List of strings to search.
    :return: The content of the last non-whitespace line or None. Cut off the search at the first line that
        starts with '****' which is used by the timestamp cryer script. The logic here is assuming that the
        cryer is configured to the same or higher period as the long running script timeout and can be used
        as a quick stop marker for identifying non-blank log lines since the last checkpoint.
    """
    last_is_timestamp = False
    for line in reversed(lines):
        if line.strip():
            if not line.startswith(b"****"):
                return line
            if last_is_timestamp:
                # Matches 2 non-blank timestamps in a row
                return None
            last_is_timestamp = True
    return None

class LongTaskMonitor:
    """
    Class representing a long-running task monitor.

    Attributes:
        complete (bool): Flag indicating if the task is complete.
        success (bool): Flag indicating if the task was successful.
        step_time (int): The time interval for the next update in seconds.
        time (int): The elapsed time for the task in seconds.

    Methods:
        cancel(): Cancels the task.
        succeeded(): Checks if the task succeeded.
        is_running(): Checks if the task is still running.
        update(): Updates the task progress and handles timeout or hang recovery.
    """

    def __init__(self, session: pexpect.spawn, max_time=5000, step_time=60):
        """
        Initializes a new instance of the LongTaskMonitor class.

        Args:
            session (pexpect.spawn): The pexpect session for the task.
            max_time (int, optional): The maximum allowed time for the task in seconds. Defaults to 5000.
            step_time (int, optional): The time interval for each update in seconds. Defaults to 60.
        """
        self.session = session
        self.complete = False
        self.success = False
        self.max_time = max_time
        self.step_time = step_time
        self.time = 0
        self.last_line = ""

    def cancel(self):
        """
        Cancels the task by sending a cancelation command to the session.
        """
        self.step_time = 5
        self.session.sendcontrol('c')
        self.session.sendline("")

    def succeeded(self) -> bool:
        """
        Checks if the task succeeded.

        Returns:
            bool: True if the task is complete and successful, False otherwise.
        """
        return self.complete and self.success

    def is_running(self) -> bool:
        """
        Checks if the task is still running.

        Returns:
            bool: True if the task is still running, False otherwise.
        """
        return not self.complete

    def update(self):
        """
        Updates the task progress and handles timeout or hang recovery.
        """
        self.time += self.step_time
        print(f"  - long running task update: {self.time}s/{self.max_time}s:")

        if not self.success and self.time > self.max_time:
            print("  - long running task exceeded max timeout, attempting cancel")
            self.cancel()
            return

        lines = self.session.before.splitlines()
        last_update_line = last_non_whitespace_line(lines)

        # print(f"-  before lines count: {len(lines)}")
        # print(f"-  old last_line: {self.last_line}")
        # print(f"-  new last_line: {last_update_line}")

        if last_update_line is None or last_update_line == self.last_line:
            print("  - detected probable interactive prompt hang, attempting recovery.")
            self.session.sendline("")
            self.session.sendline("q")
        else:
            self.last_line = last_update_line
            print(f"  - long task log[{len(lines)}]: {self.last_line}")

class AccountInitFailed(Exception):
    """
    Exception raised when the Autoinstall.init_account() fails.
    """
    def __init__(self, message="account init failed."):
        self.message = message
        super().__init__(self.message)

class ProvisionUpgradeFailed(Exception):
    """
    Exception raised when the Autoinstall.provision_upgrade() fails.
    """
    def __init__(self, message="provision upgrade failed."):
        self.message = message
        super().__init__(self.message)

class ProvisionFailed(Exception):
    """
    Exception raised when the Autoinstall.provision() fails.
    """
    def __init__(self, message="provision failed."):
        self.message = message
        super().__init__(self.message)

class DeprovisionFailed(Exception):
    """
    Exception raised when the Autoinstall.deprovision() fails.
    """
    def __init__(self, message="deprovision failed."):
        self.message = message
        super().__init__(self.message)

class AccountResetFailed(Exception):
    """
    Exception raised when the Autoinstall.reset_account() fails.
    """
    def __init__(self, message="account reset failed."):
        self.message = message
        super().__init__(self.message)

class AutoInstall:
    """
    Class representing the auto installation process.

    Methods:
        run_install(): Runs the installation process.
        run_uninstall(): Runs the uninstallation process. Requires an existing 24.05.x installation.
        run_upgrade(): Runs an upgrade installation process. Requires an existing 24.03.2 installation.

    Attributes:
        result (int): The result of the installation process. 0 indicates success, 1 indicates failure.
        result_message (str): The message describing the result of the installation process.

    Other methods or attributes are not intended for external use.
    """

    def __init__(self, install_command="./start-orchestrator-install.sh", product="full"):
        """
        Initializes a new instance of the AutoInstall class.

        Args:
            install_command (str, optional): The command to start the installation process.
            Defaults to './start-orchestrator-install.sh'.

        Raises:
            ValueError: If any required environment variables are missing.
            FileNotFoundError: If a required configuration file is not found.
        """
        self.current_step = "Initialization"

        self.cluster_name = os.getenv("CLUSTER_NAME")
        if self.cluster_name is None or len(self.cluster_name) == 0:
            raise ValueError("CLUSTER_NAME environment variable is required.")

        self.cluster_domain = os.getenv("CLUSTER_DOMAIN")
        if self.cluster_domain is None or len(self.cluster_domain) == 0:
            raise ValueError("CLUSTER_DOMAIN environment variable is required.")

        self.aws_region = os.getenv("AWS_REGION")
        if self.aws_region is None or len(self.aws_region) == 0:
            raise ValueError("AWS_REGION environment variable is required.")

        self.aws_account = os.getenv("AWS_ACCOUNT")
        if self.aws_account is None or len(self.aws_account) == 0:
            raise ValueError("AWS_ACCOUNT environment variable is required.")

        self.state_bucket_prefix = os.getenv("STATE_BUCKET_PREFIX")
        if self.state_bucket_prefix is None or len(self.state_bucket_prefix) == 0:
            raise ValueError("STATE_BUCKET_PREFIX environment variable is required.")

        self.state_path = os.getenv("STATE_PATH")
        if self.state_path is None or len(self.state_path) == 0:
            raise ValueError("STATE_PATH environment variable is required.")
        # State path must be a directory if it exists. The install container will create it if it does not exist.
        if os.path.exists(self.state_path) and not os.path.isdir(self.state_path):
            raise ValueError("STATE_PATH must be a directory.")

        # Load Proxy Settings
        self.http_proxy = os.getenv("http_proxy")
        self.https_proxy = os.getenv("https_proxy")
        self.no_proxy = os.getenv("no_proxy")
        self.socks_proxy = os.getenv("socks_proxy")

        # Handle optional test provision config settings to enable custom certs and SRE settings
        self.provision_config_path = os.getenv("USE_TEST_PROVISION_CONFIG")
        if self.provision_config_path and len(self.provision_config_path) > 0:
            if not os.path.isfile(self.provision_config_path):
                raise FileNotFoundError(f"File '{self.provision_config_path}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_config_path = (
                f"{self.state_path}/{self.aws_account}-{self.cluster_name}-values.sh"
            )
            shutil.copy(self.provision_config_path, state_config_path)

        self.provision_config_path_tfvar = os.getenv("USE_TEST_PROVISION_CONFIG_TFVAR")
        if self.provision_config_path_tfvar and len(self.provision_config_path_tfvar) > 0:
            if not os.path.isfile(self.provision_config_path_tfvar):
                raise FileNotFoundError(f"File '{self.provision_config_path_tfvar}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_config_path_tfvar= (
                f"{self.state_path}/{self.aws_account}-{self.cluster_name}-values.tfvar"
            )
            shutil.copy(self.provision_config_path_tfvar, state_config_path_tfvar)

        # Handle regsitry profile override
        self.registry_profile_path = os.getenv("AUTOINSTALL_REGISTRY_PROFILE")
        self.registry_profile_copied = False
        if self.registry_profile_path and len(self.registry_profile_path) > 0:
            if not os.path.isfile(self.registry_profile_path):
                raise FileNotFoundError(f"File '{self.registry_profile_path}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_registry_profile_path = (
                f"{self.state_path}/artifact-rs-profile.yaml"
            )
            shutil.copy(self.registry_profile_path, state_registry_profile_path)
            self.registry_profile_copied = True

        self.auto_cert = "--auto-cert"
        disable_autocert = os.getenv("DISABLE_AUTOCERT", "false")
        disable_autocert = disable_autocert.lower() == "true"
        if disable_autocert:
            self.auto_cert = ""

        self.cluster_profile = os.getenv("CLUSTER_PROFILE", "default")
        self.disable_aws_prod_profile = os.getenv("DISABLE_AWS_PROD_PROFILE", "false")

        # Init Account Settings
        self.enable_account_init = os.getenv("ENABLE_ACCOUNT_INIT")
        self.enable_account_init = self.enable_account_init.lower() == "true"
        self.enable_account_reset = os.getenv("ENABLE_ACCOUNT_RESET")
        self.enable_account_reset = self.enable_account_reset.lower() == "true"
        self.refresh_token_param = ""
        self.refresh_token = os.getenv("RS_REFRESH_TOKEN")
        if self.refresh_token is not None and len(self.refresh_token) > 0:
            self.refresh_token_param = f"--azuread-refresh-token {self.refresh_token} "

        self.rs_domain = os.getenv("RS_DOMAIN", "edgeorchestration.intel.com")
        self.rs_token_endpoint = f"https://registry-rs.{self.rs_domain}/oauth/token"

        # Support admin-roles input
        self.aws_roles = os.getenv("AWS_ROLES", "AWSReservedSSO_AWSAdministratorAccess")

        # Support for internal deployments
        self.internal = os.getenv("AUTOINSTALL_INTERNAL", "false").lower() == "true"
        self.vpc_id = os.getenv("AUTOINSTALL_VPC_ID")
        self.jumphost_ip = os.getenv("AUTOINSTALL_JUMPHOST_IP")
        self.cidr_block = os.getenv("AUTOINSTALL_CIDR_BLOCK")
        self.jumphost_sshkey_path = os.getenv("AUTOINSTALL_JUMPHOST_SSHKEY")
        self.internal_proxy_profile_path = os.getenv("AUTOINSTALL_INTERNAL_PROXY_PROFILE")
        self.internal_harbor_cert_path = os.getenv("AUTOINSTALL_INTERNAL_HARBOR_CERT")

        self.jumphost_sshkey_copied = False
        if self.jumphost_sshkey_path and len(self.jumphost_sshkey_path) > 0:
            if not os.path.isfile(self.jumphost_sshkey_path):
                raise FileNotFoundError(f"File '{self.jumphost_sshkey_path}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_config_path = (
                f"{self.state_path}/jumphost_sshkey_{self.cluster_name}"
            )
            shutil.copy(self.jumphost_sshkey_path, state_config_path)
            self.jumphost_sshkey_copied = True

        self.internal_proxy_profile_copied = False
        if self.internal_proxy_profile_path and len(self.internal_proxy_profile_path) > 0:
            if not os.path.isfile(self.internal_proxy_profile_path):
                raise FileNotFoundError(f"File '{self.internal_proxy_profile_path}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_proxy_profile_path = (
                f"{self.state_path}/proxy-internal.yaml"
            )
            shutil.copy(self.internal_proxy_profile_path, state_proxy_profile_path)
            self.internal_proxy_profile_copied = True

        # Support pre-integration flow
        self.internal_harbor_cert_copied = False
        if self.internal_harbor_cert_path and len(self.internal_harbor_cert_path) > 0:
            if not os.path.isfile(self.internal_harbor_cert_path):
                raise FileNotFoundError(f"File '{self.internal_harbor_cert_path}' not found.")
            if not os.path.exists(self.state_path):
                os.makedirs(self.state_path)
            state_internal_harbor_cert_path = (
                f"{self.state_path}/internal-harbor-ca.crt"
            )
            shutil.copy(self.internal_harbor_cert_path, state_internal_harbor_cert_path)
            self.internal_harbor_cert_copied = True

        self.custom_vpc = self.vpc_id is not None and len(self.vpc_id) > 0
        if self.internal and not self.custom_vpc:
            raise ValueError("Internal deployments require a custom VPC ID.")

        if self.custom_vpc:
            if self.jumphost_ip is None or len(self.jumphost_ip) == 0:
                raise ValueError("Custom VPC deployments require a jumphost IP address.")
            if self.cidr_block is None or len(self.cidr_block) == 0:
                raise ValueError("Custom VPC deployments require a CIDR block.")
            if not self.jumphost_sshkey_copied:
                raise ValueError("Custom VPC deployments require a valid jumphost SSH key.")

        if self.internal and self.enable_account_init:
            raise ValueError("Account initialization is not supported for internal deployments.")

        # Default Internal and VPC parameters
        self.internal_params = ""
        self.vpc_params = ""
        self.vpc_jumphost_params = ""
        self.internal_makefile_params = ""
        self.socks_proxy_params = ""
        self.internal_rs_url = "registry-rs.edgeorchestration.intel.com"
        self.enable_cache_registry = "--enable-cache-registry"

        if self.internal:
            self.internal_params = "--internal"
            self.internal_makefile_params = "USE_REPO_PROXY=true USE_INTERNAL_PROXY=true USE_TEST_ADMIN=true"
            # Check if internal RS and update makefile params
            if self.rs_domain in self.internal_rs_url:
                self.internal_makefile_params += " USE_INTERNAL_REGISTRY_CERTS=true"

            # TODO: do we really need to set the cluster DNS IP? Why?
            self.eks_internal_params = "--eks-cluster-dns-ip 172.20.0.10 "
            if self.http_proxy:
                self.eks_internal_params += f"--eks-http-proxy {self.http_proxy} "
            if self.https_proxy:
                self.eks_internal_params += f"--eks-https-proxy {self.https_proxy} "
            if self.no_proxy:
                self.eks_internal_params += f"--eks-no-proxy \"{self.no_proxy}\" "

        if self.custom_vpc:
            self.vpc_params = f"--skip-apply-vpc --vpc-id {self.vpc_id}"
            self.vpc_jumphost_params = f"--jumphost-ip {self.jumphost_ip} --cidr-block {self.cidr_block}"

        # Verify Proxy Settings
        if not self.internal:
            if self.https_proxy and len(self.https_proxy):
                if self.socks_proxy is None or len(self.socks_proxy) == 0:
                    raise ValueError("The socks_proxy environment variable must be set in a proxied network environment.")
                self.socks_proxy_params = f"--socks-proxy {self.socks_proxy}"

        # Development/Debugging switches
        # Set ENABLE_TIMING_DATA to capture timing data for progress estimate development
        self.enable_timing_data = os.getenv("ENABLE_TIMING_DATA")
        self.enable_timing_data = self.enable_timing_data.lower() == "true"
        # Set ENABLE_INTERACTIVE_DEBUG to cause fatal exceptions to drop to an interactive operation mode.
        self.enable_interactive_debug = os.getenv("ENABLE_INTERACTIVE_DEBUG")
        self.enable_interactive_debug = self.enable_interactive_debug.lower() == "true"

        # Cleanup Required State
        self.deprovision_on_cleanup = False
        self.reset_account_on_cleanup = False

        # Install session result, assume unknown error (-1)
        self.result_message = None
        self.result = -1

        self.product = product
        self.mode = "install"
        self.log_file_path = os.getenv("AUTO_INSTALL_LOGPATH")
        if self.log_file_path is None:
            timestamp = time.strftime("%Y%m%d-%H%M%S")
            self.log_file_path = f"../install-{timestamp}.log"
        self.log_file = None
        self.installer_session = None
        self.install_command = install_command

        if not self.internal:
            enable_cache_registry = os.getenv("ENABLE_CACHE_REGISTRY", "false")
            if enable_cache_registry.lower() == "true":
                self.enable_cache_registry = "--enable-cache-registry"

    def start_installer(self, installer_option=1):
        """
        Start the installer and handle pre-container launch settings prompts for installation.

        Raises:
            pexpect.exceptions.TIMEOUT: If a timeout occurs while handling pre-container settings prompts.
            pexpect.exceptions.EOF: If an EOF error occurs while handling pre-container settings prompts.
        """

        self.current_step = "Start Installer"
        print(f"Step: {self.current_step}")

        # Select an installation option
        print(f"  - Set install option to {installer_option}")
        self.installer_session.expect(r"Your selection \(default \[.*\]\):")
        self.installer_session.sendline(f"{installer_option}")
        # Select a cluster name
        print("  - Set cluster name to " + str(self.cluster_name))
        self.installer_session.expect(r"Enter the name of the cluster \[.*\]:")
        self.installer_session.sendline(str(self.cluster_name))
        # Select a region
        print("  - Set AWS region to " + str(self.aws_region))
        self.installer_session.expect(r"Specify the AWS region for the cluster \(default \[.*\]\):")
        self.installer_session.sendline(str(self.aws_region))
        # local state step
        print("  - Set state data prefix to " + str(self.state_bucket_prefix))
        self.installer_session.expect(r"Specify the state data identifier for the cluster \(default \[.*\]\):")
        self.installer_session.sendline(str(self.state_bucket_prefix))
        # local state step
        print("  - Set state path to " + str(self.state_path))
        self.installer_session.expect(r"local state path \(default \[.*\]\):")
        self.installer_session.sendline(str(self.state_path))

        # Wait for the orchestrator-admin container prompt. May take a few minutes to launch the
        # container before the shell prompt comes up.
        print("  - Await installer container startup")
        self.installer_session.expect("orchestrator-admin:~", timeout=600)

        # Note: This timeout will need to be extended when the wrapper script is pulling the
        # image from the release service. The image pull can take 10 minutes or more depending
        # on network performance. Add a specific pull timeout/progress watch phase to verify pull
        # progress when it is being done here.

    def start_time_cryer(self):
        """
        Run shellcryer.sh to capture timing data for progress estimate development.

        Raises:
            pexpect.exceptions.TIMEOUT: If a timeout occurs while awaiting the orchestrator-admin prompt.
            pexpect.exceptions.EOF: If an EOF error occurs while awaiting the orchestrator-admin prompt.
        """

        self.current_step = "Start Time Cryer"
        print(f"Step: {self.current_step}")

        if self.enable_timing_data:
            self.installer_session.sendline("./pod-configs/SAVEME/shellcryer.sh &")
            self.installer_session.expect("orchestrator-admin:.*")

    def init_account(self):
        """
        Initializes the AWS account infrastructure for the installation account.
        """

        self.current_step = "Init Account"
        print(f"Step: {self.current_step}")

        if self.enable_account_init:
            self.installer_session.sendline("cd ~/pod-configs")
            self.installer_session.expect("orchestrator-admin:pod-configs")

            # This must be set prior to the provision.sh call as both successful and partial failed
            # account init attempts need to be cleaned up on failure.
            self.reset_account_on_cleanup = True
            self.installer_session.sendline(
                f"utils/provision.sh account --new-aws-account "
                f"--aws-account {self.aws_account} "
                f"--region $AWS_REGION "
                f"--customer-state-prefix {self.state_bucket_prefix} "
                f"{self.refresh_token_param} "
                f"--azuread-token-endpoint {self.rs_token_endpoint} --auto"
            )

            # Handle long run account init checking
            long_task = LongTaskMonitor(self.installer_session, 2000, 60)
            while long_task.is_running():
                match_id = self.installer_session.expect(
                    [
                        pexpect.TIMEOUT,
                        r"Info: The AWS account is initialized\.",
                        r"orchestrator-admin:pod-configs",
                        r"to terminate the running shuttle process and continue the provision\."
                    ],
                    timeout=long_task.step_time,
                )

                if match_id == 0:
                    # handle long task step
                    long_task.update()

                elif match_id == 1:
                    # handle success indicator found
                    long_task.success = True
                    long_task.step_time=10

                elif match_id == 2:
                    # handle prompt found (complete state)
                    long_task.complete = True

                elif match_id == 3:
                    # Handle shuttle disconnect prompt
                    self.installer_session.sendline("yes")

            if not long_task.succeeded():
                raise AccountInitFailed()

    def configure_provision(self):
        """
        Configures the provision settings for the installation process.
        """

        self.current_step = "Configure Provision"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~/pod-configs")
        self.installer_session.expect("orchestrator-admin:pod-configs")
        self.installer_session.sendline(
            f"utils/provision.sh config --aws-account {self.aws_account} "
            f"--customer-state-prefix {self.state_bucket_prefix} "
            f"--environment $CLUSTER_NAME --parent-domain {self.cluster_domain} "
            f"--region $AWS_REGION --email builder@infra-host.com "
            f"--profile {self.cluster_profile} "
            f"{self.auto_cert} {self.internal_params} {self.vpc_params} {self.vpc_jumphost_params} {self.socks_proxy_params} --auto "
            f"{self.enable_cache_registry} "
            f"{self.eks_internal_params}"
        )

        # Wait for editor to open. This should be an i/o match, but that is not working and the context
        # download may take a minute or so before the editor starts up.
        time.sleep(120)
        # in provision config editor
        self.installer_session.sendline(":wq")

        # Confirm config save if prompted
        match_id = self.installer_session.expect(
            [
                "to save it and proceed, others to quit:",
                "Info: Values are saved. Execute the config command to update them if it is needed.",
            ]
        )
        if match_id == 0:
            print("  - confirm config save")
            self.installer_session.sendline("yes")

        self.installer_session.expect("orchestrator-admin:pod-configs", timeout=60)

    def provision_upgrade(self):
        """
        Performs the provisioning upgrade step of the installation process.
        """

        self.current_step = "Provision Upgrade"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~/pod-configs")

        self.installer_session.expect("orchestrator-admin:pod-configs")
        self.installer_session.sendline(
            f"utils/provision.sh upgrade --aws-account {self.aws_account} "
            f"--customer-state-prefix {self.state_bucket_prefix} "
            f"--environment $CLUSTER_NAME --parent-domain {self.cluster_domain} "
            f"--region $AWS_REGION --email builder@infra-host.com "
            f"--aws-admin-roles {self.aws_roles} "
            f"--azuread-refresh-token {self.refresh_token} "
            f"--profile {self.cluster_profile} "
            f"{self.auto_cert} {self.internal_params} {self.vpc_params} {self.vpc_jumphost_params} {self.socks_proxy_params} --auto "
            f"{self.enable_cache_registry} "
            f"{self.eks_internal_params}"
        )

        # Handle long run provision upgrade checking
        long_task = LongTaskMonitor(self.installer_session, 14500, 60)
        while long_task.is_running():
            match_id = self.installer_session.expect(
                [
                    pexpect.TIMEOUT,
                    r"Info: The upgrade completed successfully\.",
                    r"orchestrator-admin:pod-configs",
                    r"Enter \'yes\' to start the upgrades\. Enter others to exit:",
                    r"to terminate the running shuttle process and continue the provision\."
                ],
                timeout=long_task.step_time,
            )

            if match_id == 0:
                # handle long task step
                long_task.update()

            elif match_id == 1:
                # handle success indicator found
                long_task.success = True
                long_task.step_time=10

            elif match_id == 2:
                # handle prompt found (complete state)
                long_task.complete = True

            elif match_id == 3:
                # Handle upgrade confirm prompt
                self.installer_session.sendline("yes")

            elif match_id == 4:
                # Handle shuttle disconnect prompt
                self.installer_session.sendline("yes")

        if not long_task.succeeded():
            raise ProvisionUpgradeFailed()

    def provision(self):
        """
        Performs the provisioning step of the installation process.
        """

        self.current_step = "Provision"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~/pod-configs")

        self.installer_session.expect("orchestrator-admin:pod-configs")
        self.installer_session.sendline(
            f"utils/provision.sh install --aws-account {self.aws_account} "
            f"--customer-state-prefix {self.state_bucket_prefix} "
            f"--environment $CLUSTER_NAME --parent-domain {self.cluster_domain} "
            f"--region $AWS_REGION --email builder@infra-host.com "
            f"--aws-admin-roles {self.aws_roles} "
            f"--azuread-refresh-token {self.refresh_token} "
            f"--profile {self.cluster_profile} "
            f"{self.auto_cert} {self.internal_params} {self.vpc_params} {self.vpc_jumphost_params} {self.socks_proxy_params} --auto --reduce-ns-ttl "
            f"{self.enable_cache_registry} "
            f"{self.eks_internal_params}"
        )

        # Handle long run install checking
        long_task = LongTaskMonitor(self.installer_session, 5000, 60)
        while long_task.is_running():
            match_id = self.installer_session.expect(
                [
                    pexpect.TIMEOUT,
                    r"Info: Installation completed successfully\. Please back up the files in .* directory\.",
                    r"orchestrator-admin:pod-configs",
                    r"to terminate the running shuttle process and continue the provision\."
                ],
                timeout=long_task.step_time,
            )

            if match_id == 0:
                # handle long task step
                print('  - handle long task step - check for timeout and handle cancel - stuck shell')
                try:
                    long_task.update()
                except Exception as err:
                    print (f"long_task_monitor: exception in update: {err}")

            elif match_id == 1:
                # handle success indicator found
                print('  - success found, marking task successful')
                long_task.success = True
                long_task.step_time=10

            elif match_id == 2:
                # handle prompt found (complete attempt)
                print('  - prompt found, marking task complete')
                long_task.complete = True

            elif match_id == 3:
                # Handle shuttle disconnect prompt
                self.installer_session.sendline("yes")

        if not long_task.succeeded():
            raise ProvisionFailed()

    def configure_cluster(self):
        """
        Configures the cluster settings for the installation process.
        """
        self.current_step = "Configure Cluster"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        # Support registry profile override for non-prod deployments
        if self.registry_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/artifact-rs-profile.yaml ~/edge-manageability-framework/config/profiles/artifact-rs-production-noauth.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # replace internal proxy profile for internal deployments
        if self.internal and self.internal_proxy_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/proxy-internal.yaml ~/edge-manageability-framework/config/profiles/proxy-none.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # configure cluster
        self.installer_session.sendline(f"DISABLE_AWS_PROD_PROFILE={self.disable_aws_prod_profile} ./configure-cluster.sh {self.vpc_jumphost_params}")

        editor_prompt = False
        while not editor_prompt:
            match_id = self.installer_session.expect(
                [
                    r"Enter the full domain name for the cluster:",
                    r"Please provide the administrator email address associated with the cluster's provisioning:",
                    r"Press any key to open your editor"
                ],
                timeout=360)

            if match_id == 0:
                # handle FQDN missing prompt
                self.installer_session.sendline(f"{self.cluster_name}.{self.cluster_domain}\n")

            elif match_id == 1:
                # handle ADMIN_EMAIL missing prompt
                self.installer_session.sendline("builder@infra-host.com\n")

            elif match_id == 2:
                # handle editor prompt, enter to proceed
                self.installer_session.sendline("\n")
                editor_prompt = True

        # deploy config edit
        time.sleep(5)
        self.installer_session.sendline(":wq")

        # makefile step
        self.installer_session.expect("orchestrator-admin:~")

    def prepare_upgrade(self):
        """
        Performs pre-provisioning upgrade tasks to ensure apps are up to date.
        """
        self.current_step = "Prepare Upgrade"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        # Support registry profile override for non-prod deployments
        if self.registry_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/artifact-rs-profile.yaml ~/edge-manageability-framework/config/profiles/artifact-rs-production-noauth.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # replace internal proxy profile for internal deployments
        if self.internal and self.internal_proxy_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/proxy-internal.yaml ~/edge-manageability-framework/config/profiles/proxy-none.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # configure cluster
        self.installer_session.sendline(f"./prepare-upgrade.sh {self.vpc_jumphost_params}")

        command_prompt = False
        while not command_prompt:
            match_id = self.installer_session.expect(
                [
                    r"Enter the full domain name for the cluster:",
                    r"Please provide the administrator email address associated with the cluster's provisioning:",
                    r"orchestrator-admin:~"
                ],
                timeout=360)

            if match_id == 0:
                # handle FQDN missing prompt
                self.installer_session.sendline(f"{self.cluster_name}.{self.cluster_domain}\n")

            elif match_id == 1:
                # handle ADMIN_EMAIL missing prompt
                self.installer_session.sendline("builder@infra-host.com\n")

            elif match_id == 2:
                # handle editor prompt, enter to proceed
                command_prompt = True

        # wait for app pre-provision upgrade to complete
        # TBD: add a script that can monitor the app upgrade progress and provide a timeout
        time.sleep(120)

    def update_cluster(self):
        """
        Performs pre-deployment container inititialization to enable cluster update and management operations.
        """
        self.current_step = "Update Cluster"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        # Support registry profile override for non-prod deployments
        if self.registry_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/artifact-rs-profile.yaml ~/edge-manageability-framework/config/profiles/artifact-rs-production-noauth.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # replace internal proxy profile for internal deployments
        if self.internal and self.internal_proxy_profile_copied:
            self.installer_session.sendline(f"cp pod-configs/SAVEME/proxy-internal.yaml ~/edge-manageability-framework/config/profiles/proxy-none.yaml")
            self.installer_session.expect("orchestrator-admin:~")

        # configure cluster
        self.installer_session.sendline(f"./update-cluster.sh {self.vpc_jumphost_params}")

        command_prompt = False
        while not command_prompt:
            match_id = self.installer_session.expect(
                [
                    r"Enter the full domain name for the cluster:",
                    r"Please provide the administrator email address associated with the cluster's provisioning:",
                    r"orchestrator-admin:~"
                ],
                timeout=360)

            if match_id == 0:
                # handle FQDN missing prompt
                self.installer_session.sendline(f"{self.cluster_name}.{self.cluster_domain}\n")

            elif match_id == 1:
                # handle ADMIN_EMAIL missing prompt
                self.installer_session.sendline("builder@infra-host.com\n")

            elif match_id == 2:
                # handle editor prompt, enter to proceed
                command_prompt = True

        # wait for app pre-provision upgrade to complete
        # TBD: add a script that can monitor the app upgrade progress and provide a timeout
        time.sleep(120)


    def makefile_install(self):
        """
        Performs the installation using the makefile.
        """
        self.current_step = "Make Install"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        # create registry cert configmap for pre-integration deployments
        if self.internal and self.rs_domain in self.internal_rs_url and self.internal_harbor_cert_copied:
            self.installer_session.sendline(f"kubectl create configmap registry-certs -n argocd --from-file=registry-certs.crt=/root/pod-configs/SAVEME/internal-harbor-ca.crt")
            self.installer_session.expect("orchestrator-admin:~")

        self.installer_session.sendline(f"{self.internal_makefile_params} make install")

        # installation complete - get argocd password - TBD select one approach to caching this and remove the other.
        self.installer_session.expect("orchestrator-admin:~", timeout=600)
        self.installer_session.sendline(
            'echo "export ARGO_ADMIN_PASS=$(kubectl -n argocd get secret argocd-initial-admin-secret '
            '-o jsonpath="{.data.password}" | base64 -d)" > pod-configs/SAVEME/argoauth.env'
        )

        self.installer_session.expect("orchestrator-admin:~")
        self.installer_session.sendline(
            'echo -e "{\n  "argoAuth": "$(kubectl -n argocd get secret argocd-initial-admin-secret '
            '-o jsonpath="{.data.password}" | base64 -d)"\n}" > pod-configs/SAVEME/argoauth.json'
        )

        self.installer_session.expect("orchestrator-admin:~")

    def makefile_update(self):
        """
        Updates the installation using the makefile.
        """
        self.current_step = "Make Update"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        self.installer_session.sendline(f"{self.internal_makefile_params} make update")
        self.installer_session.expect("orchestrator-admin:~", timeout=60)

    def makefile_upgrade(self):
        """
        Updates the installation using the makefile.
        """
        self.current_step = "Make Upgrade"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~")
        self.installer_session.expect("orchestrator-admin:~")

        self.installer_session.sendline(f"{self.internal_makefile_params} make upgrade")
        self.installer_session.expect("orchestrator-admin:~", timeout=600)

    def deprovision(self):
        """
        This function deprovisioning cloud resources with options required to work around known partial
        install cleanup issues. The uninstall is a best effort and can fail out without full cleanup if
        all known automated uninstall operations fail.
        """
        self.current_step = "Deprovision"
        print(f"Step: {self.current_step}")

        self.installer_session.sendline("cd ~/pod-configs")

        retry_attempt = 0
        skip_options = [
            "--skip-destroy-loadbalancer",
        ]
        uninstall_command = (
            f"utils/provision.sh uninstall --aws-account {self.aws_account} "
            f"--customer-state-prefix {self.state_bucket_prefix} --environment $CLUSTER_NAME "
            f"--parent-domain {self.cluster_domain} --region $AWS_REGION --email builder@infra-host.com "
            f"--aws-admin-roles {self.aws_roles} "
            f"--azuread-refresh-token {self.refresh_token} "
            f"--profile {self.cluster_profile} "
            f"{self.auto_cert} {self.internal_params} {self.vpc_params} {self.vpc_jumphost_params} {self.socks_proxy_params} --auto "
            f"{self.enable_cache_registry}"
        )

        self.installer_session.expect("orchestrator-admin:pod-configs")
        while True:
            self.installer_session.sendline(uninstall_command)

            # Handle long run uninstall checking
            long_task = LongTaskMonitor(self.installer_session, 4000, 60)
            while long_task.is_running():
                match_id = self.installer_session.expect(
                    [
                        pexpect.TIMEOUT,
                        r"Info: Uninstallation completed successfully\.",
                        r"orchestrator-admin:pod-configs",
                        r"to terminate the running shuttle process and continue the provision\.",
                    ],
                    timeout=long_task.step_time,
                )

                if match_id == 0:
                    # handle long task step
                    long_task.update()

                elif match_id == 1:
                    # handle success indicator found
                    long_task.success = True
                    long_task.step_time = 10

                elif match_id == 2:
                    # handle prompt found (attempt complete state)
                    long_task.complete = True

                elif match_id == 3:
                    # Handle shuttle disconnect prompt
                    self.installer_session.sendline("yes")

            if not long_task.succeeded() and (retry_attempt < len(skip_options)):
                uninstall_command = (
                    uninstall_command + " " + skip_options[retry_attempt]
                )
                print(f"retry deprovision with: {uninstall_command}")
                self.installer_session.sendline(uninstall_command)
                retry_attempt += 1
            else:
                if not long_task.succeeded():
                    raise DeprovisionFailed()
                break

    def reset_account(self):
        """
        Resets the AWS account infrastructure for the installation account.
        """

        self.current_step = "Reset Account"
        print(f"Step: {self.current_step}")

        if self.enable_account_reset or self.reset_account_on_cleanup:
            # Flush the input buffer

            output = b''
            while True:
                try:
                    chunk = self.installer_session.read_nonblocking(size=1024, timeout=0.1)
                    output += chunk
                except pexpect.TIMEOUT:
                    break
                except pexpect.EOF:
                    break

            # match_id=self.installer_session.expect([pexpect.TIMEOUT, pexpect.EOF], timeout=2)
            # if match_id != 0:
            #     raise AccountResetFailed()

            # Set current directory to ~/pod-configs
            self.installer_session.sendline("cd ~/pod-configs")
            self.installer_session.expect("orchestrator-admin:pod-configs")

            max_retries = 2
            retry_attempt = 0
            while True:
                self.installer_session.sendline(
                    f"utils/provision.sh account --reset-aws-account "
                    f"--aws-account {self.aws_account} "
                    f"--region $AWS_REGION "
                    f"--customer-state-prefix {self.state_bucket_prefix} "
                    f"--azuread-refresh-token {self.refresh_token} "
                    f"--azuread-token-endpoint {self.rs_token_endpoint} --auto"
                )

                # Handle long run provision upgrade checking
                long_task = LongTaskMonitor(self.installer_session, 1000, 60)
                while long_task.is_running():
                    match_id = self.installer_session.expect(
                        [
                            pexpect.TIMEOUT,
                            r"Info: The AWS account is reset\.",
                            r"orchestrator-admin:pod-configs",
                            r"to terminate the running shuttle process and continue the provision\."
                        ],
                        timeout=long_task.step_time,
                    )

                    if match_id == 0:
                        # handle long task step
                        long_task.update()

                    elif match_id == 1:
                        # handle success indicator found
                        print("matched success")
                        long_task.success = True
                        long_task.step_time=10

                    elif match_id == 2:
                        # handle prompt found (complete state)
                        print("matched complete")
                        long_task.complete = True

                    elif match_id == 3:
                        # Handle shuttle disconnect prompt
                        print("matched shuttle disconnect request")
                        self.installer_session.sendline("yes")

                if not long_task.succeeded() and (retry_attempt < max_retries):
                    print(f"retry account reset")
                    retry_attempt += 1
                else:
                    if not long_task.succeeded():
                        print("exited long_task without success")
                        raise AccountResetFailed()
                    break

    def cleanup_on_failure(self):
        """
        Cleans up the failed installation by uninstalling the provisioned resources and resetting the account.
        """

        self.current_step = "Cleanup on Failure"

        if self.deprovision_on_cleanup:
            self.deprovision()
        if self.reset_account_on_cleanup:
            self.reset_account()

    def exit_install_container(self):
        """
        Exits the install container and await the install script closure.
        """

        self.current_step = "Exit Install Container"

        self.installer_session.sendline("exit")
        match_id = self.installer_session.expect(
            [
                pexpect.TIMEOUT,
                pexpect.EOF
            ],
            timeout=30,
        )

        if match_id == 0:
            # One exit TIMEOUT retry attempt before closing the session
            self.installer_session.sendline("exit")

        self.installer_session.wait()

    def run_install(self):
        """
        Runs the installation process.
        """

        # Create a log file for the install session
        self.mode = "install"

        # Create a log file for the install session
        self.log_file = open(self.log_file_path, "wb")

        # Start the orchestrator install session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file

        try:
            option = 1
            self.start_installer(installer_option=option)
            self.start_time_cryer()
            self.init_account()
            self.configure_provision()
            # This must be set prior to the provision.sh call as both successful and partial failed
            # provision attempts need to be cleaned up on overall AutoInstall failure. This has to be
            # set by run_install() as an upgrade provision must not be deprovisioned on failure.
            self.deprovision_on_cleanup = False
            self.provision()
            self.configure_cluster()
            self.makefile_install()
            self.exit_install_container()

            self.result = 0
            self.result_message = (
                f"auto_install: {self.mode} operation completed successfully."
            )

        except pexpect.exceptions.TIMEOUT as err:
            self.result = 2 # Timeout
            self.result_message = (
                f"auto_install: {self.mode} failed with a TIMEOUT during {self.current_step}: {err}"
            )
            # print(self.result_message)
            if self.enable_interactive_debug:
                self.installer_session.interact() # Give control of the self.installer_session to the user.

        except pexpect.exceptions.EOF as err:
            self.result = 3 # EOF
            self.result_message = (
                f"auto_install: {self.mode} failed with an EOF during {self.current_step}: {err}"
            )
            # print(self.result_message)

            # EOF indicates the stdout on the spawned process closed. Close out the session and logs
            # so they can be reopened when/if a recover or cleanup attempt is made.
            self.installer_session.close()
            self.log_file.close()

        except Exception as err:
            self.result = 1 # Unknown Error
            self.result_message = f"auto_install: {self.mode} failed with an exception: {err}"
            # print(self.result_message)

        # TBD: add self.rollback_upgrade() to handle upgrade failures in the future
        # Attempt Cleanup on Failure
        if self.result == 1:
            self.cleanup_on_failure()
        if self.result == 2:
            if self.handle_timeout_recovery():
                self.cleanup_on_failure()
        elif self.result == 3:
            if self.handle_eof_recovery():
                self.cleanup_on_failure()

    def run_uninstall(self):
        """
        Runs the uninstallation process.
        """

        self.mode = "uninstall"

        # Create a log file for the install session
        self.log_file = open(self.log_file_path, "wb")

        # Start the orchestrator install session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file

        try:
            option = 3

            self.start_installer(installer_option=option)
            self.start_time_cryer()
            self.deprovision()
            self.reset_account()
            self.exit_install_container()

            self.result = 0
            self.result_message = (
                f"auto_install: {self.mode} operation completed successfully."
            )
            return

        except pexpect.exceptions.TIMEOUT as err:
            self.result = 2 # Timeout
            self.result_message = (
                f"auto_install: {self.mode} failed with a TIMEOUT during {self.current_step}: {err}"
            )
            # print(self.result_message)
            if self.enable_interactive_debug:
                self.installer_session.interact() # Give control of the self.installer_session to the user.

        except pexpect.exceptions.EOF as err:
            self.result = 3 # EOF
            self.result_message = (
                f"auto_install: {self.mode} failed with an EOF during {self.current_step}: {err}"
            )
            # print(self.result_message)

        except Exception as err:
            self.result = 1 # Unknown Error
            self.result_message = f"auto_install: {self.mode} failed with an exception: {err}"
            #print(self.result_message)

        # There is no additional handling for deprovisioning exceptions. Known recovery handling is incorporated
        # into the deprovision() method.

    def run_upgrade(self):
        """
        Runs the upgrade install workflow.

        This depends on an existing deployment of an upgrade supported previous version deployed to the
        specified cloud environment.
        """

        # Create a log file for the install session
        self.log_file = open(self.log_file_path, "wb")

        # Start the orchestrator install session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file

        try:
            self.mode = "upgrade"
            option = 2

            self.start_installer(installer_option=option)
            self.start_time_cryer()
            self.prepare_upgrade()
            self.configure_provision()
            # TBD: An upgrade provisioning failure can't be cleaned up with a deprovision. The upgrade
            # process should build in a rollback option that handles the expected backup and restore steps
            # that customers will use to recover from a failed upgrade.
            self.provision_upgrade()
            self.configure_cluster()
            self.makefile_upgrade()
            self.exit_install_container()

            self.result = 0 # Success
            self.result_message = (
                f"auto_install: {self.mode} operation completed successfully."
            )
            return

        except pexpect.exceptions.TIMEOUT as err:
            self.result = 2 # Timeout
            self.result_message = (
                f"auto_install: {self.mode} failed with a TIMEOUT during {self.current_step}: {err}"
            )
            # print(self.result_message)
            if self.enable_interactive_debug:
                self.installer_session.interact() # Give control of the self.installer_session to the user.

        except pexpect.exceptions.EOF as err:
            self.result = 3 # EOF
            self.result_message = (
                f"auto_install: {self.mode} failed with an EOF during {self.current_step}: {err}"
            )
            # print(self.result_message)

            # EOF indicates the stdout on the spawned process closed. Close out the session and logs
            # so they can be reopened when/if a recover or cleanup attempt is made.
            self.installer_session.close()
            self.log_file.close()

        except Exception as err:
            self.result = 1 # Unknown Error
            self.result_message = f"auto_install: {self.mode} failed with an exception: {err}"
            # print(self.result_message)

        # TBD: add self.rollback_upgrade() to handle upgrade failures in the future
        # # Attempt Rollback Upgrade
        # if self.result == 1:
        #     self.rollback_upgrade()
        # if self.result == 2:
        #     if self.handle_timeout_recovery():
        #         self.rollback_upgrade()
        # elif self.result == 3:
        #     if self.handle_eof_recovery(installer_option=option):
        #         self.rollback_upgrade()

    def run_update_cluster_setting(self):
        """
        Runs the update cluster re-install workflow.

        This depends on an existing deployment of the same version deployed to the specified cloud environment.
        The process reprovisions the cluster, updates cluster configuration, and updates the application to apply
        the updated cluster configuration.
        """

        # Create a log file for the install session
        self.log_file = open(self.log_file_path, "wb")

        # Start the orchestrator install session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file

        try:
            self.mode = "update-cluster-setting"
            option = 2

            self.start_installer(installer_option=option)
            self.start_time_cryer()
            self.update_cluster()
            self.configure_provision()
            self.deprovision_on_cleanup = False
            self.provision()
            self.configure_cluster()
            self.makefile_update()
            self.exit_install_container()

            self.result = 0 # Success
            self.result_message = (
                f"auto_install: {self.mode} operation completed successfully."
            )
            return

        except pexpect.exceptions.TIMEOUT as err:
            self.result = 2 # Timeout
            self.result_message = (
                f"auto_install: {self.mode} failed with a TIMEOUT during {self.current_step}: {err}"
            )
            # print(self.result_message)
            if self.enable_interactive_debug:
                self.installer_session.interact() # Give control of the self.installer_session to the user.

        except pexpect.exceptions.EOF as err:
            self.result = 3 # EOF
            self.result_message = (
                f"auto_install: {self.mode} failed with an EOF during {self.current_step}: {err}"
            )
            # print(self.result_message)

            # EOF indicates the stdout on the spawned process closed. Close out the session and logs
            # so they can be reopened when/if a recover or cleanup attempt is made.
            self.installer_session.close()
            self.log_file.close()

        except Exception as err:
            self.result = 1 # Unknown Error
            self.result_message = f"auto_install: {self.mode} failed with an exception: {err}"
            # print(self.result_message)

        # TBD: add self.rollback_upgrade() to handle upgrade failures in the future
        # # Attempt Rollback Upgrade
        # if self.result == 1:
        #     self.rollback_upgrade()
        # if self.result == 2:
        #     if self.handle_timeout_recovery():
        #         self.rollback_upgrade()
        # elif self.result == 3:
        #     if self.handle_eof_recovery(installer_option=option):
        #         self.rollback_upgrade()

    def run_update(self):
        """
        Runs the application deploy update workflow.

        This depends on an existing deployment of a previous application version deployed to the
        specified cloud environment. It will only update the application without reprovisioning the
        cluster.
        """

        # Create a log file for the install session
        self.log_file = open(self.log_file_path, "wb")

        # Start the orchestrator install session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file

        try:
            self.mode = "update"
            option = 2

            self.start_installer(installer_option=option)
            self.start_time_cryer()
            self.update_cluster()
            self.configure_cluster()
            self.makefile_update()
            self.exit_install_container()

            self.result = 0 # Success
            self.result_message = (
                f"auto_install: {self.mode} operation completed successfully."
            )
            return

        except pexpect.exceptions.TIMEOUT as err:
            self.result = 2 # Timeout
            self.result_message = (
                f"auto_install: {self.mode} failed with a TIMEOUT during {self.current_step}: {err}"
            )
            # print(self.result_message)
            if self.enable_interactive_debug:
                self.installer_session.interact() # Give control of the self.installer_session to the user.

        except pexpect.exceptions.EOF as err:
            self.result = 3 # EOF
            self.result_message = (
                f"auto_install: {self.mode} failed with an EOF during {self.current_step}: {err}"
            )
            # print(self.result_message)

            # EOF indicates the stdout on the spawned process closed. Close out the session and logs
            # so they can be reopened when/if a recover or cleanup attempt is made.
            self.installer_session.close()
            self.log_file.close()

        except Exception as err:
            self.result = 1 # Unknown Error
            self.result_message = f"auto_install: {self.mode} failed with an exception: {err}"
            # print(self.result_message)

    def handle_timeout_recovery(self):
        """
        Handle a timeout error during an install/upgrade workflow.

        Returns:
            bool: True if the recovery was successful, False otherwise.
        """
        match_id = -1
        retry = 5
        while (match_id < 1) and (retry > 0):
            retry -= 1
            self.installer_session.sendcontrol("c")
            self.installer_session.sendline("")
            match_id = self.installer_session.expect(
                [
                    pexpect.TIMEOUT,
                    "orchestrator-admin:.*",
                ],
                timeout=5
            )
        return match_id == 1

    def handle_eof_recovery(self, installer_option=1):
        """
        Handle an EOF an install/upgrade workflow.

        Returns:
            bool: True if the recovery was successful, False otherwise.
        """
        # Reopen the log file for append
        self.log_file = open(self.log_file_path, "ab")
        self.log_file.write("\n\n---\nEOF during upgrade. Cleanup log follows:\n---\n\n".encode())

        # Restart the installer session
        self.installer_session = pexpect.spawn(self.install_command)
        self.installer_session.logfile = self.log_file
        try:
            self.start_installer(installer_option=installer_option)
            return True
        except Exception as err:
            print(f"Failed to restart installer session: {err}")
            return False

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description="Script to automate workflows for the Edge Orchestrator cloud installer within Jenkins E2E "
            "validation pipelines.",
        epilog="environment variables:\n\n"
        "Most parameters to the script are supplied through OS environment variables as it integrates well with "
        "Jenkins pipeline implementation.\n\n"
        "  automation workflow parameters:\n"
        "    - AUTO_INSTALL_LOGPATH: The path for the installer execution output log file.\n"
        "    - ENABLE_TIMING_DATA: Flag indicating whether timing data is enabled. (true/false)\n"
        "    - ENABLE_INTERACTIVE_DEBUG: Flag indicating whether interactive debugging is enabled. (true/false)\n"
        "  cluster installation parameters:\n"
        "    - AWS_REGION: AWS region for the installed cluster.\n"
        "    - AWS_ACCOUNT: AWS account for the installed cluster.\n"
        "    - CLUSTER_NAME: The name of the cluster.\n"
        "    - CLUSTER_DOMAIN: The parent domain of the cluster.\n"
        "    - STATE_BUCKET_PREFIX: shared state bucket for the cluster being installed\n"
        "    - STATE_PATH: local path mounted in the installer container to store shared state data\n"
        "  custom cluster configuration support parameters:\n"
        "    - USE_TEST_PROVISION_CONFIG: Path to the test provision config file.\n"
        "    - DISABLE_AUTOCERT: Flag indicating whether auto certificate is disabled. (true/false)\n"
        "    - CLUSTER_PROFILE: The scaling profile for cluster(default, 100en, 1ken, 10ken)\n"
        "  account initialization workflow support:\n"
        "    - ENABLE_ACCOUNT_INIT: Flag to specify whather account initialization is run on install. (true/false)\n"
        "    - ENABLE_ACCOUNT_RESET: Flag to specify whether account reset is run on uninstall. (true/false)\n"
        "    - RS_REFRESH_TOKEN: The refresh token for the account initialization workflow. (optional)\n"
        "    - RS_TOKEN_ENDPOINT: The release service endpoint to obtain refresh token for the account initialization workflow.\n"
        "  proxy settings required by the install environment:\n"
        "    - https_proxy: HTTPS proxy\n"
        "    - socks_proxy: SOCKS proxy.\n")
    parser.add_argument(
        "-m",
        "--mode",
        choices=["install", "upgrade", "update", "update-cluster-setting", "uninstall"],
        default="install",
        help="Specify the mode (install, upgrade, update, update-cluster-setting, uninstall)",
    )
    parser.add_argument(
        "-p",
        "--product",
        choices=["full"],
        default="full",
        help="Specify the product SKU (full)",
    )
    args = parser.parse_args()

    try:
        # auto_installer = AutoInstall("./fake-install.sh", product=args.product)
        auto_installer = AutoInstall(product=args.product)
    except Exception as outer_err:
        print(f"failed to initialize Installer workflow object: {outer_err}\n")
        parser.print_help()
        sys.exit(1)

    if args.mode == "install":
        auto_installer.run_install()
    elif args.mode == "upgrade":
        auto_installer.run_upgrade()
    elif args.mode == "update":
        auto_installer.run_update()
    elif args.mode == "update-cluster-setting":
        auto_installer.run_update_cluster_setting()
    elif args.mode == "uninstall":
        auto_installer.run_uninstall()

    if auto_installer.result_message:
        print(auto_installer.result_message)
    sys.exit(auto_installer.result)
