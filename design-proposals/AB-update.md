# Design Proposal: A/B Update of Edge Microvisor Toolkit - Standalone (EMT-S)

Author(s): Edge Infrastructure Manager Team

Last updated: 06/05/2025

## Abstract

This document is a design proposal for the A/B Partition Day 2 update of an Edge Node (EN) with an immutable Microvisor OS.
This design proposal aims to enhance the Microvisor OS update process for edge nodes, ensuring that new features can be
delivered without requiring end users to reinstall their software.

## Proposal

For the immutable Microvisor OS, the OS packages are part of the image, and the way to update the OS packages is by providing
a new OS image with updated package versions. To achieve this, two read-only partitions will be created: the A and B partitions.
The A partition will persist the original OS installation, and the B partition will be used to install a new OS.
Depending on the success of the updated OS installation, the OS will boot from the new partition (B) or
roll back to the original partition (A) in case of failure.

## Rationale

We are developing a new script that calls the os-update-tool, rather than modifying and expanding the os-update-tool itself to
include the functionality needed for pulling an image and executing the update.

## Affected Components and Teams

We report hereafter the affected components and teams:

- EMT (Edge Microvisor Toolkit team)
- EMT-S (Edge Microvisor Toolkit - Standalone)

## Implementation Plan

To perform the A/B upgrade procedure for an immutable Microvisor OS, follow the steps below:

**Step 1:** Admin logs in to the EMT-S EN and executes the script located at `/etc/cloud/os-update.sh`. This script is responsible
for initiating the OS update process.

**Step 2:** `/etc/cloud/os-update.sh` requires two arguments:

- The URL or USB mount path where the desired OS image is located.
- The URL or USB mount path for the SHA file corresponding to the OS image, which is used for integrity verification.

**Sample commands:**  

- Direct Path Command:
  `os-update.sh /mnt/edge-readonly-3.0.20250518.2200-signed.raw.gz 8aadeadc9a44af9703de59604b12364534b32c34408d087e2358fecc87ef277a`
  `os-update.sh /home/user/edge-readonly-3.0.20250518.2200-signed.raw.gz 8aadeadc9a44af9703de59604b12364534b32c34408d087e2358fecc87ef277a`
- URL Command:
  `os-update.sh -u <url_to_microvisor_os_image> <url_to_sha_file>`

**Step 3:** The script `/etc/cloud/os-update.sh` calls another script `/usr/bin/os-update-tool.sh` to perform the actual update procedure.

- Execute the update tool script with the write command to write into the inactive partition:  
  `os-update-tool.sh -w -u <file_path_to_EMT_image> -s <check_sum_value>`
- Execute the update tool with the apply command to set the newly written image to be used for the next boot:  
  `os-update-tool.sh -a`
- Reboot: Restart the system to boot from the newly applied OS image.

**Step 4:** Upon successful boot, verify that the system is running correctly with the new image:

- `sudo bootctl list`

- Make the new image persistent for future boots using the following command:  
  `os-update-tool.sh -c`

> **Note:** Step 4 is the only step that can be integrated into the cloud-init script.

![Immutable OS Update flow](./images/A_B-Upgrade.png)

## Test Plan

Following tests have been planned to verify this feature

1. Update the Edge Node with the latest version of the Microvisor OS
2. Update the Edge Node with the older version of the Microvisor OS
3. Provision the EN with a specific profile (EMT-NRT) that includes k3s and Docker, and then update the OS
4. Test the system's ability to handle unexpected or incorrect OS versions and its fallback mechanism
