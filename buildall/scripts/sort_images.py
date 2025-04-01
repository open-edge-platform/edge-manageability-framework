#!/usr/bin/env python

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
sort_builds.py
combines artifact and manifest information into datastructure that can be used
to do new builds
"""

import re
from ba_lib import load_yaml, load_repo_artifacts

# globals
repo_tags_to_build = {}


def parse_image_manifest(image_mf_raw):
    """parse the combined repo/registry/image/version in image manifest"""

    image_mf_fix = {"images": {}}

    for rawi in image_mf_raw["images"]:

        registry = ""
        name = ""
        version = ""

        # print(f"original: {rawi}")

        # handle sha256 specified images
        if "@" in rawi:
            split = re.findall(
                r"([a-z0-9-/\.]*)/([a-z0-9-]*)(@sha256:[a-z0-9-\.]*)", rawi
            )
            # print(f"sha split: {split}")
            registry, name, version = split[0]

        else:
            # this works for most longer images, but fails on short ones missing registry
            split = re.findall(r"([a-z0-9-/\.]*)/([a-z0-9-]*):([a-z0-9-\.]*)", rawi)

            if len(split) == 1:
                # print(f"n split: {split}")
                registry, name, version = split[0]

            # handle registry free tags
            else:
                split = re.findall(r"([a-z-]*):([a-z0-9-\.]*)", rawi)
                # print(f"rf split: {split}")

                if len(split) == 1:
                    name, version = split[0]
                else:
                    print(f"Underspecified image name: {rawi}")

        if name in image_mf_fix["images"]:
            if image_mf_fix["images"][name]["registry"] != registry:
                print(
                    f"Duplicate image: '{name}' has registry '{registry}' which "
                    f"doesn't match earlier '{image_mf_fix['images'][name]['registry']}'"
                )
            else:
                if version not in image_mf_fix["images"][name]["versions"]:
                    image_mf_fix["images"][name]["versions"].append(version)
        else:
            image_mf_fix["images"][name] = {"registry": registry, "versions": [version]}

    return image_mf_fix


def match_images(image_mf, image_to_repo, artifacts):
    """match up images in manifest with where they are built in repos"""

    for image, repo in image_to_repo.items():
        if image in image_mf["images"]:

            repo_image_data = artifacts[repo]["images"][image]
            manifest_image_data = image_mf["images"][image]

            for mv in manifest_image_data["versions"]:
                image_tag_to_build = repo_image_data["gitTagPrefix"] + mv

                if "buildTarget" in repo_image_data:
                    image_tag_to_build += "|" + repo_image_data["buildTarget"]

                if repo in repo_tags_to_build:
                    repo_tags_to_build[repo]["images"].append(image_tag_to_build)
                else:
                    repo_tags_to_build[repo] = {"charts": [], "images": []}
                    repo_tags_to_build[repo]["images"].append(image_tag_to_build)


if __name__ == "__main__":

    image_raw = load_yaml("scratch/manifest_images.yaml")
    image_manifest = parse_image_manifest(image_raw)

    repo_artifacts, _ctr, itr = load_repo_artifacts("scratch")

    match_images(image_manifest, itr, repo_artifacts)

    print("builds needed in each repo")
    print(repo_tags_to_build)

    for rname, data in repo_tags_to_build.items():
        if data["images"]:  # If there are charts in the repo

            # write tags into a file, one per line
            with open(f"scratch/itags_{rname}", "w", encoding="utf-8") as itagout:
                for line in data["images"]:
                    itagout.write(f"{line}\n")
