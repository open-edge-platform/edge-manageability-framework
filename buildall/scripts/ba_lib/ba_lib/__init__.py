# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
ba_lib.py - shared buildall functions
"""

import re
from pathlib import Path
from ruamel.yaml import YAML

# yaml parser (common)
yaml = YAML(typ="safe")


def load_yaml(filename):
    """given a filename, returns a dict with file contents parsed as YAML"""

    with open(filename, "r", encoding="utf-8") as infile:
        loaded = yaml.load(infile)
    return loaded


def load_repo_artifacts(repodir):
    """given a directory, load artifacts yaml and put in repo_artifacts dict"""

    repo_artifacts = {}
    chart_to_repo = {}
    image_to_repo = {}

    p = Path(repodir)

    for artifact_fn in p.glob("artifacts_*.yaml"):
        with open(artifact_fn, "r", encoding="utf-8") as af:
            repo_name = re.findall(r"artifacts_([a-z0-9-]+).yaml", str(artifact_fn))[0]
            repo_data = yaml.load(af)
            repo_artifacts[repo_name] = repo_data

            if "charts" in repo_data:
                for chart in repo_data["charts"]:
                    chart_to_repo[chart] = repo_name

            if "images" in repo_data:
                for image in repo_data["images"]:
                    image_to_repo[image] = repo_name

    return repo_artifacts, chart_to_repo, image_to_repo
